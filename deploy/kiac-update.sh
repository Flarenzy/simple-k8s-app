#!/usr/bin/env bash
set -euo pipefail

context="${KIAC_CONTEXT:-kiac-dev}"
namespace="${KIAC_NAMESPACE:-ipam}"
release="${KIAC_RELEASE:-ipam}"
chart="${KIAC_CHART:-deploy/helm/ipam}"
image_tag="${KIAC_IMAGE_TAG:-}"
pull_policy="${KIAC_IMAGE_PULL_POLICY:-Always}"
discovery_site_id="${KIAC_DISCOVERY_SITE_ID:-}"
discovery_namespaces="${KIAC_DISCOVERY_NAMESPACES:-default,ipam}"
keycloak_host="${KIAC_KEYCLOAK_HOST:-}"
postgres_secret="${KIAC_POSTGRES_SECRET:-ipam-postgres-postgresql}"
postgres_workload="${KIAC_POSTGRES_WORKLOAD:-statefulset/ipam-postgres-postgresql}"

for command in git helm kubectl openssl; do
	command -v "$command" >/dev/null || { echo "Required command not found: $command" >&2; exit 1; }
done

kubectl config get-contexts "$context" >/dev/null 2>&1 || { echo "Kubernetes context $context does not exist" >&2; exit 1; }
helm status "$release" --namespace "$namespace" --kube-context "$context" >/dev/null 2>&1 || {
	echo "Helm release $release is not installed in namespace $namespace on $context" >&2
	exit 1
}

if [[ -z "$image_tag" ]]; then
	image_tag="$(git ls-remote --exit-code origin refs/heads/main | awk 'NR == 1 { print $1 }')"
fi
[[ -n "$image_tag" ]] || { echo "Unable to resolve the latest merged image SHA" >&2; exit 1; }
api_image_tag="${KIAC_API_IMAGE_TAG:-$image_tag}"
fe_image_tag="${KIAC_FE_IMAGE_TAG:-$image_tag}"
migrations_image_tag="${KIAC_MIGRATIONS_IMAGE_TAG:-$image_tag}"
api_pull_policy="${KIAC_API_IMAGE_PULL_POLICY:-$pull_policy}"
fe_pull_policy="${KIAC_FE_IMAGE_PULL_POLICY:-$pull_policy}"
migrations_pull_policy="${KIAC_MIGRATIONS_IMAGE_PULL_POLICY:-$pull_policy}"

frontend_route="$release-fe"
frontend_host="$(kubectl --context "$context" --namespace "$namespace" get httproute "$frontend_route" -o jsonpath='{.spec.hostnames[0]}')"
api_host="$(kubectl --context "$context" --namespace "$namespace" get httproute "$release-api" -o jsonpath='{.spec.hostnames[0]}')"
gateway_name="$(kubectl --context "$context" --namespace "$namespace" get httproute "$frontend_route" -o jsonpath='{.spec.parentRefs[0].name}')"
gateway_namespace="$(kubectl --context "$context" --namespace "$namespace" get httproute "$frontend_route" -o jsonpath='{.spec.parentRefs[0].namespace}')"
gateway_section="$(kubectl --context "$context" --namespace "$namespace" get httproute "$frontend_route" -o jsonpath='{.spec.parentRefs[0].sectionName}')"
gateway_namespace="${gateway_namespace:-$namespace}"
gateway_protocol="$(kubectl --context "$context" --namespace "$gateway_namespace" get gateway "$gateway_name" -o "jsonpath={.spec.listeners[?(@.name=='$gateway_section')].protocol}")"
[[ -n "$frontend_host" && -n "$gateway_protocol" ]] || { echo "The existing frontend route is not attached to a Gateway listener" >&2; exit 1; }

if [[ -z "$keycloak_host" ]]; then
	keycloak_host="$(kubectl --context "$context" --namespace "$namespace" get httproute "$release-keycloak" -o jsonpath='{.spec.hostnames[0]}' 2>/dev/null || true)"
fi
keycloak_host="${keycloak_host:-keycloak.simplek8sapp.lan}"

case "$gateway_protocol" in
	HTTPS|https) scheme="https" ;;
	HTTP|http) scheme="http" ;;
	*) echo "Unsupported Gateway listener protocol: $gateway_protocol" >&2; exit 1 ;;
esac
keycloak_url="${KIAC_KEYCLOAK_URL:-$scheme://$keycloak_host}"

require_secret_key() {
	local secret_name="$1"
	local secret_key="$2"
	local value
	value="$(kubectl --context "$context" --namespace "$namespace" get secret "$secret_name" -o "jsonpath={.data.$secret_key}" 2>/dev/null || true)"
	[[ -n "$value" ]] || { echo "Required Secret $namespace/$secret_name is missing key $secret_key" >&2; exit 1; }
}

ensure_generated_secret() {
	local secret_name="$1"
	shift
	if kubectl --context "$context" --namespace "$namespace" get secret "$secret_name" >/dev/null 2>&1; then
		for secret_key in "$@"; do
			require_secret_key "$secret_name" "$secret_key"
		done
		return
	fi
	local password
	password="$(openssl rand -base64 32)"
	if [[ "$secret_name" == "keycloak-bootstrap" ]]; then
		printf '%s' "$password" | kubectl --context "$context" --namespace "$namespace" create secret generic "$secret_name" --from-literal=username=keycloak-bootstrap --from-file=password=/dev/stdin >/dev/null
	else
		printf '%s' "$password" | kubectl --context "$context" --namespace "$namespace" create secret generic "$secret_name" --from-file=password=/dev/stdin >/dev/null
	fi
	echo "Created local Secret $namespace/$secret_name"
}

require_secret_key "ipam-db" "DB_CONN"
ensure_generated_secret "keycloak-bootstrap" username password
ensure_generated_secret "ipam-application-admin" password

if kubectl --context "$context" --namespace "$namespace" get secret keycloak-db >/dev/null 2>&1; then
	require_secret_key keycloak-db password
else
	postgres_password="$(kubectl --context "$context" --namespace "$namespace" get secret "$postgres_secret" -o jsonpath='{.data.password}' 2>/dev/null || true)"
	[[ -n "$postgres_password" ]] || { echo "PostgreSQL Secret $namespace/$postgres_secret is missing key password" >&2; exit 1; }
	printf '%s' "$postgres_password" | base64 -d | kubectl --context "$context" --namespace "$namespace" create secret generic keycloak-db --from-file=password=/dev/stdin >/dev/null
	unset postgres_password
	echo "Created local Secret $namespace/keycloak-db from the existing PostgreSQL credential"
fi

kubectl --context "$context" --namespace "$namespace" create configmap ipam-realm --from-file=ipam-realm.json=dev/example-prod-realm.json --dry-run=client -o yaml | kubectl --context "$context" apply -f - >/dev/null

if [[ -z "$discovery_site_id" ]]; then
	discovery_site_id="$(kubectl --context "$context" --namespace "$namespace" get deployment "$release-api" -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="KUBERNETES_DISCOVERY_SITE_ID")].value}' 2>/dev/null || true)"
fi
if [[ -z "$discovery_site_id" ]]; then
	site_ids="$(kubectl --context "$context" --namespace "$namespace" exec "$postgres_workload" -- sh -lc 'export PGPASSWORD="$(cat "$POSTGRES_PASSWORD_FILE")"; psql -U "$POSTGRES_USER" -d "$POSTGRES_DATABASE" -Atc "select id::text from sites order by created_at"')"
	site_count="$(printf '%s\n' "$site_ids" | awk 'NF { count++ } END { print count + 0 }')"
	if [[ "$site_count" != "1" ]]; then
		echo "Set KIAC_DISCOVERY_SITE_ID to an existing site UUID; automatic selection requires exactly one site (found $site_count)" >&2
		exit 1
	fi
	discovery_site_id="$(printf '%s\n' "$site_ids" | awk 'NF { print; exit }')"
fi
[[ "$discovery_site_id" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]] || { echo "Invalid discovery site UUID: $discovery_site_id" >&2; exit 1; }

discovery_json='['
IFS=',' read -r -a requested_namespaces <<< "$discovery_namespaces"
for index in "${!requested_namespaces[@]}"; do
	requested_namespace="${requested_namespaces[$index]}"
	if [[ "$requested_namespace" != "*" && ! "$requested_namespace" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
		echo "Invalid discovery namespace: $requested_namespace" >&2
		exit 1
	fi
	[[ "$index" == "0" ]] || discovery_json+=','
	discovery_json+="\"$requested_namespace\""
done
discovery_json+=']'

helm upgrade --install "$release" "$chart" \
	--namespace "$namespace" \
	--kube-context "$context" \
	--reuse-values \
	--rollback-on-failure \
	--timeout "${KIAC_DEPLOY_TIMEOUT:-10m}" \
	--set-string "api.image.tag=$api_image_tag" \
	--set-string "api.image.pullPolicy=$api_pull_policy" \
	--set-string "fe.image.tag=$fe_image_tag" \
	--set-string "fe.image.pullPolicy=$fe_pull_policy" \
	--set-string "migrations.image.tag=$migrations_image_tag" \
	--set-string "migrations.image.pullPolicy=$migrations_pull_policy" \
	--set api.auth.enabled=true \
	--set-string "api.auth.issuer=$keycloak_url/realms/ipam" \
	--set-string api.auth.audience=ipam-api \
	--set-string "api.auth.jwksURL=http://$release-keycloak:8080/realms/ipam/protocol/openid-connect/certs" \
	--set api.kubernetesDiscovery.enabled=true \
	--set-string api.kubernetesDiscovery.sourceKey=kiac-dev \
	--set-string "api.kubernetesDiscovery.sourceName=kiac development" \
	--set-string "api.kubernetesDiscovery.siteID=$discovery_site_id" \
	--set-json "api.kubernetesDiscovery.namespaces=$discovery_json" \
	--set-string "fe.env.VITE_KEYCLOAK_URL=$keycloak_url" \
	--set-string fe.env.VITE_KEYCLOAK_REALM=ipam \
	--set-string fe.env.VITE_KEYCLOAK_CLIENT_ID=ipam-fe \
	--set-string fe.env.VITE_KEYCLOAK_ROLE_CLIENT_ID=ipam-api \
	--set keycloak.enabled=true \
	--set-string keycloak.admin.existingSecret=keycloak-bootstrap \
	--set-string keycloak.applicationAdmin.existingSecret=ipam-application-admin \
	--set-string keycloak.db.existingSecret=keycloak-db \
	--set-string "keycloak.hostname.url=$keycloak_url" \
	--set keycloak.realmImport.enabled=true \
	--set-string keycloak.realmImport.configMapName=ipam-realm \
	--set-string "httpRoute.keycloak.hostnames[0]=$keycloak_host"

echo "kiac deployment updated"
echo "  context: $context"
echo "  release: $namespace/$release"
echo "  API image tag: $api_image_tag ($api_pull_policy)"
echo "  frontend image tag: $fe_image_tag ($fe_pull_policy)"
echo "  migrations image tag: $migrations_image_tag ($migrations_pull_policy)"
echo "  Gateway listener: $gateway_namespace/$gateway_name:$gateway_section ($gateway_protocol)"
echo "  frontend origin: $scheme://$frontend_host"
echo "  Keycloak URL: $keycloak_url"
echo "  discovery site: $discovery_site_id"
echo "  discovery namespaces: $discovery_namespaces"
echo "Retrieve the local application-admin password with:"
echo "  kubectl --context $context -n $namespace get secret ipam-application-admin -o jsonpath='{.data.password}' | base64 -d; echo"

gateway_address="$(kubectl --context "$context" --namespace "$gateway_namespace" get gateway "$gateway_name" -o jsonpath='{.status.addresses[0].value}')"
unresolved_hosts=()
for hostname in "$frontend_host" "$api_host" "$keycloak_host"; do
	if command -v dscacheutil >/dev/null 2>&1; then
		dscacheutil -q host -a name "$hostname" | grep -q "ip_address: $gateway_address" || unresolved_hosts+=("$hostname")
	elif command -v getent >/dev/null 2>&1; then
		getent hosts "$hostname" | grep -q "$gateway_address" || unresolved_hosts+=("$hostname")
	fi
done
if [[ "${#unresolved_hosts[@]}" -gt 0 ]]; then
	echo "Workstation DNS is missing the Gateway address for: ${unresolved_hosts[*]}" >&2
	echo "Add this local hosts entry before browser validation:" >&2
	echo "  $gateway_address $frontend_host $api_host $keycloak_host" >&2
fi
