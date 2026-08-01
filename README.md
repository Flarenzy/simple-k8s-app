# Simple IPAM application
The point of this repo is to learn on how to deploy a application on a minikube cluster.
The idea is to have a Kubernetes ingress, a NGINX or similar proxy for statically serving the frontend, golang API for all CRUD operations, Keycloak for authorization and authentification and Postgres as a persistant data store.
Database migrations will be handeled by `goose` and `sqlc` will be used for type safe queries.

## Deploying to Minikube (Podman driver)

Prereqs: `minikube` (with the ingress addon), `helm`, access to GHCR images, and Postgres 16+ in the cluster (see below).

1. Enable ingress in minikube (keep the tunnel running or port-forward the controller):
   ```bash
   minikube addons enable ingress
   minikube tunnel   # keep this terminal open; will prompt for sudo
   ```

2. Install Postgres 16 via Helm (Bitnami) and create the DB secret:
   ```bash
   export POSTGRES_PASSWORD="yourpassword"
   helm upgrade --install ipam-postgres bitnami/postgresql \
     -n ipam --create-namespace \
     --set auth.username=ipam \
     --set auth.password=$POSTGRES_PASSWORD \
     --set auth.database=ipam

   kubectl -n ipam create secret generic ipam-db \
     --from-literal=DB_CONN="postgres://ipam:${POSTGRES_PASSWORD}@ipam-postgres-postgresql.ipam.svc.cluster.local:5432/ipam?sslmode=disable"
   ```

3. Deploy the app chart:
   ```bash
   helm upgrade --install ipam deploy/helm/ipam -n ipam \
     --set db.existingSecret=ipam-db \
     --set ingress.enabled=true \
     --set ingress.className=nginx \
     --set ingress.hosts[0].host=ipam.local
   ```

4. Add a host entry for the app and access it:
   ```text
   <minikube-ip> ipam.local
   ```
   Then open `http://ipam.local/` (FE) and `http://ipam.local/api/v1/healthz` (API).

Notes:
- Migration hook runs as a Helm post-install/upgrade job using the `ipam-migrate` image; ensure `DB_CONN` secret exists before deploying.
- Images are published to GHCR: `ghcr.io/flarenzy/ipam-api`, `ghcr.io/flarenzy/ipam-fe`, `ghcr.io/flarenzy/ipam-migrate`.
- CI now publishes `linux/amd64` and `linux/arm64` manifests for the first-party images. On Apple Silicon, use immutable SHA tags plus `imagePullPolicy=Always` while validating fresh builds so the cluster does not reuse a cached `latest`.
- Tagged releases also publish a signed OCI Helm chart to `oci://ghcr.io/flarenzy/charts/ipam`. The matching `.tgz`, `.prov`, and public signing key are attached to the GitHub Release so consumers can verify provenance with `helm verify`. The public key is committed at `docs/helm-release-public.asc`; for the full verification flow, see `docs/helm-release-verification.md`.

## Deploying to kiac (Apple Containers)

kiac provides Kubernetes nodes as Apple container VMs and includes a Traefik Gateway API stack when created with `--gateway`. The workflow below uses separate local hostnames for the frontend and API.

Prereqs: Apple Silicon Mac, `container`, `kiac`, `kubectl`, `helm`, and an account with access to the repository's GHCR images.

1. Create the cluster with Gateway API enabled:
   ```bash
   kiac create cluster --name dev --workers 1 --gateway
   kubectl config use-context kiac-dev
   ```

2. Install Postgres and create the application DB secret:
   ```bash
   export POSTGRES_PASSWORD="yourpassword"
   helm upgrade --install ipam-postgres bitnami/postgresql \
     -n ipam --create-namespace \
     --set auth.username=ipam \
     --set auth.password="$POSTGRES_PASSWORD" \
     --set auth.database=ipam

   kubectl -n ipam create secret generic ipam-db \
     --from-literal=DB_CONN="postgres://ipam:${POSTGRES_PASSWORD}@ipam-postgres-postgresql.ipam.svc.cluster.local:5432/ipam?sslmode=disable"
   ```

3. Push to `main` and wait for the CI `build-and-push` job to publish multi-architecture images. Use the commit SHA tag rather than `latest`:
   ```bash
   export IMAGE_TAG="<commit-sha>"
   ```

4. Deploy the chart through kiac's Gateway. The Gateway created by kiac is named `kiac` in namespace `kiac-gateway`:
   ```bash
   helm upgrade --install ipam deploy/helm/ipam \
     -n ipam --create-namespace \
     --set db.existingSecret=ipam-db \
     --set httpRoute.enabled=true \
     --set 'httpRoute.parentRefs[0].name=kiac' \
     --set 'httpRoute.parentRefs[0].namespace=kiac-gateway' \
     --set 'httpRoute.parentRefs[0].sectionName=http' \
     --set-string "api.image.tag=$IMAGE_TAG" \
     --set api.image.pullPolicy=Always \
     --set-string "fe.image.tag=$IMAGE_TAG" \
     --set fe.image.pullPolicy=Always \
     --set-string "migrations.image.tag=$IMAGE_TAG" \
     --set migrations.image.pullPolicy=Always \
     --set-string 'api.env.CORS_ALLOWED_ORIGINS=http://simplek8sapp.lan' \
     --set-string 'fe.env.VITE_API_BASE=http://api.simplek8sapp.lan/api/v1'
   ```

5. Add the Traefik LoadBalancer address to `/etc/hosts`:
   ```bash
   kubectl get svc -n kiac-gateway traefik
   ```
   Then add its `EXTERNAL-IP`:
   ```text
   <gateway-ip> simplek8sapp.lan api.simplek8sapp.lan
   ```

   Open `http://simplek8sapp.lan/` and check the API at `http://api.simplek8sapp.lan/healthz`.

The kiac Gateway exposes HTTP by default. HTTPS requires configuring a TLS certificate and an HTTPS listener on the Gateway. Because the frontend and API use different hostnames, the API allows the frontend origin through `CORS_ALLOWED_ORIGINS`.

### HTTPS on kiac with mkcert

To use HTTPS locally, install and trust the mkcert CA, create one certificate for both application hostnames, and store it in the Gateway namespace:

```bash
mkcert -install
CERT_DIR="$(mktemp -d)"
mkcert -cert-file "$CERT_DIR/simplek8sapp.pem" \
  -key-file "$CERT_DIR/simplek8sapp-key.pem" \
  simplek8sapp.lan api.simplek8sapp.lan

kubectl -n kiac-gateway create secret tls simplek8sapp-tls \
  --cert="$CERT_DIR/simplek8sapp.pem" \
  --key="$CERT_DIR/simplek8sapp-key.pem" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Add an HTTPS listener to kiac's Gateway. Leaving `hostname` unset allows both names on the certificate, including the apex frontend hostname:

```bash
kubectl -n kiac-gateway patch gateway kiac --type=json -p='[{"op":"add","path":"/spec/listeners/-","value":{"name":"https","port":443,"protocol":"HTTPS","tls":{"mode":"Terminate","certificateRefs":[{"kind":"Secret","name":"simplek8sapp-tls"}]},"allowedRoutes":{"namespaces":{"from":"All"}}}}]'
```

Redeploy the application routes and browser origins against the HTTPS listener:

```bash
helm upgrade --install ipam deploy/helm/ipam -n ipam --create-namespace \
  --reuse-values \
  --set-string 'httpRoute.parentRefs[0].name=kiac' \
  --set-string 'httpRoute.parentRefs[0].namespace=kiac-gateway' \
  --set-string 'httpRoute.parentRefs[0].sectionName=https' \
  --set-string 'api.env.CORS_ALLOWED_ORIGINS=https://simplek8sapp.lan' \
  --set-string 'fe.env.VITE_API_BASE=https://api.simplek8sapp.lan/api/v1'
```

Then open `https://simplek8sapp.lan/`. The API health check is available at `https://api.simplek8sapp.lan/healthz`.

### HTTPS certificate runbook

Check the certificate currently served by the Kubernetes Secret:

```bash
kubectl -n kiac-gateway get secret simplek8sapp-tls
kubectl -n kiac-gateway get secret simplek8sapp-tls \
  -o jsonpath='{.data.tls\.crt}' | base64 -D | \
  openssl x509 -noout -subject -issuer -dates -ext subjectAltName
```

Check that the Gateway and both routes are ready:

```bash
kubectl -n kiac-gateway get gateway kiac -o wide
kubectl -n kiac-gateway get gateway kiac \
  -o jsonpath='{range .status.listeners[*]}{.name}{": programmed="}{.conditions[?(@.type=="Programmed")].status}{", routes="}{.attachedRoutes}{"\n"}{end}'
kubectl -n ipam get httproute ipam-fe ipam-api -o wide
```

Verify the certificate and application from the workstation:

```bash
openssl s_client -connect simplek8sapp.lan:443 \
  -servername simplek8sapp.lan </dev/null 2>/dev/null | \
  openssl x509 -noout -dates -subject -issuer
curl -fsS https://simplek8sapp.lan/ >/dev/null && echo "frontend: OK"
curl -fsS https://api.simplek8sapp.lan/healthz && echo " api: OK"
```

Refresh an expired or soon-to-expire certificate by generating a new leaf certificate and applying it to the existing Secret. The Gateway listener references the Secret by name, so it does not need to be patched again:

```bash
CERT_DIR="$(mktemp -d)"
mkcert -cert-file "$CERT_DIR/simplek8sapp.pem" \
  -key-file "$CERT_DIR/simplek8sapp-key.pem" \
  simplek8sapp.lan api.simplek8sapp.lan

kubectl -n kiac-gateway create secret tls simplek8sapp-tls \
  --cert="$CERT_DIR/simplek8sapp.pem" \
  --key="$CERT_DIR/simplek8sapp-key.pem" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Wait for Traefik to observe the updated Secret, then rerun the status and HTTPS checks above. If the certificate is still rejected by the browser, reinstall the local CA with `mkcert -install` and restart the browser.

## Local Dev (Compose + Keycloak)

- The recommended local dev stack is:
  1. `make dev-up`
  2. `make db-migrate`
  3. `make run`
  4. open `http://localhost:5173`
- `make dev-up` starts Postgres and Keycloak from `dev/docker-compose.yaml` on:
  - `localhost:5432`
  - `localhost:8080`
- `make run` starts:
  - the API on `localhost:4040`
  - the frontend on `localhost:5173`
- `make run` now wires local auth automatically:
  - API:
    - `AUTH_ENABLED=true`
    - `KEYCLOAK_ISSUER=http://localhost:8080/realms/ipam`
    - `KEYCLOAK_AUDIENCE=ipam-api`
    - `KEYCLOAK_JWKS_URL=http://localhost:8080/realms/ipam/protocol/openid-connect/certs`
  - Frontend:
    - `VITE_KEYCLOAK_URL=http://localhost:8080`
    - `VITE_KEYCLOAK_REALM=ipam`
    - `VITE_KEYCLOAK_CLIENT_ID=ipam-fe`
    - `VITE_API_BASE=/api/v1`
- The local Keycloak realm import comes from `dev/ipam-realm.json` and is scoped to `localhost:5173` / `127.0.0.1:5173`.

## Kubernetes Service discovery

Kubernetes discovery is an optional, read-only enrichment process. It lists core `v1/Service` objects, derives `service.namespace.svc.<cluster-domain>` names, and associates ClusterIPs and literal LoadBalancer ingress IPs only with existing IPAM addresses in the configured site. It never creates or deletes IPAM rows and never changes the manually maintained `hostname` field.

Discovery is disabled by default. Enable it with these API environment variables:

| Variable | Default | Meaning |
| --- | --- | --- |
| `KUBERNETES_DISCOVERY_ENABLED` | `false` | Enables the client and background runner. |
| `KUBERNETES_DISCOVERY_SOURCE_KEY` | none | Stable operator-chosen cluster/source identity. |
| `KUBERNETES_DISCOVERY_SOURCE_NAME` | source key | Display name returned by the API. |
| `KUBERNETES_DISCOVERY_SITE_ID` | none | Existing IPAM site UUID used as the exact-match boundary. |
| `KUBERNETES_DISCOVERY_AUTH_MODE` | `in_cluster` | `in_cluster` or `kubeconfig`. |
| `KUBERNETES_DISCOVERY_KUBECONFIG_PATH` | none | Explicit path required in `kubeconfig` mode; there is no home-directory fallback. |
| `KUBERNETES_DISCOVERY_KUBECONFIG_CONTEXT` | kubeconfig current context | Optional explicit context in `kubeconfig` mode. |
| `KUBERNETES_DISCOVERY_NAMESPACES` | none | Comma-separated namespace names, or `*` by itself. |
| `KUBERNETES_DISCOVERY_CLUSTER_DOMAIN` | `cluster.local` | Suffix used for derived Service DNS names. |
| `KUBERNETES_DISCOVERY_INTERVAL` | `5m` | Complete-snapshot reconciliation interval. |
| `KUBERNETES_DISCOVERY_REQUEST_TIMEOUT` | `15s` | Deadline for a complete Kubernetes list and status writes. |
| `KUBERNETES_DISCOVERY_STALE_RETENTION` | `168h` | Retention for inactive Service observations before cleanup. |

For local discovery against the `kiac` context, first create the target site in IPAM, then run the API with an explicit kubeconfig:

```bash
export KUBERNETES_DISCOVERY_ENABLED=true
export KUBERNETES_DISCOVERY_SOURCE_KEY=kiac-dev
export KUBERNETES_DISCOVERY_SOURCE_NAME='kiac development'
export KUBERNETES_DISCOVERY_SITE_ID='<existing-site-uuid>'
export KUBERNETES_DISCOVERY_AUTH_MODE=kubeconfig
export KUBERNETES_DISCOVERY_KUBECONFIG_PATH="$KUBECONFIG"
export KUBERNETES_DISCOVERY_KUBECONFIG_CONTEXT=kiac-dev
export KUBERNETES_DISCOVERY_NAMESPACES=default,ipam
make run-api
```

If `KUBECONFIG` is unset, supply the explicit file path instead. The production Helm path always uses in-cluster ServiceAccount credentials. A named namespace scope creates only `get`, `list`, and `watch` access to core Services in those namespaces; an explicit `*` scope creates the equivalent cluster-wide grant:

```bash
helm upgrade --install ipam deploy/helm/ipam -n ipam \
  --set api.kubernetesDiscovery.enabled=true \
  --set api.kubernetesDiscovery.sourceKey=kiac-prod \
  --set api.kubernetesDiscovery.siteID='<existing-site-uuid>' \
  --set 'api.kubernetesDiscovery.namespaces[0]=default'
```

Every IP response contains a non-null `kubernetes_services` array. `GET /api/v1/kubernetes/sources` is protected by the same bearer-token boundary as the other application routes and reports `pending`, `healthy`, or `degraded` source state. A Service is linked only when its address has exactly one match inside the bound site; zero matches remain unmatched and overlapping matches remain ambiguous. Headless and ExternalName Services, pending LoadBalancers, and hostname-only LoadBalancer ingress are retained without invented IP associations.

A failed namespace list, timeout, RBAC denial, malformed observed address, or persistence failure never publishes a partial snapshot. The last successful Service associations remain available, `/readyz` continues to check PostgreSQL only, and the source status becomes degraded. A later complete empty snapshot is authoritative and marks the previous Services inactive.

Validation commands:

```bash
make test
make test-integration
npm --prefix frontend run build
make sqlc
make docs
helm lint deploy/helm/ipam
helm template ipam deploy/helm/ipam
```

## Integration Tests

Run the PostgreSQL- and Keycloak-backed API suite with:

```sh
make test-integration
```

The command automatically uses a running Apple Container service on Apple silicon macOS, or Testcontainers with Docker/Podman elsewhere. Apple Container requires macOS 26 or later; install its signed package, then run `container system start`. Select a provider explicitly when both are installed:

```sh
INTEGRATION_CONTAINER_RUNTIME=apple make test-integration
INTEGRATION_CONTAINER_RUNTIME=testcontainers make test-integration
```

The test output identifies the selected and detected runtimes. It fails with setup instructions when none is usable. See [the integration-test runtime guide](integration/context.md#container-runtime-compatibility) for image overrides, startup timeout configuration, registry authentication, cleanup, Apple Silicon behavior, and Docker/Podman fallback details.

## Optional: Keycloak

- The supported Minikube + Keycloak topology uses two explicit hosts:
  - `ipam.local` for the frontend and API
  - `keycloak.local` for the Keycloak ingress
- For current Keycloak releases, the recommended local setup is HTTPS on both ingresses. Use local TLS secrets (for example via `mkcert`) and keep `keycloak.proxy.headers=xforwarded` so Keycloak trusts the forwarded host/scheme headers from ingress-nginx.
- Do not leave `keycloak.ingress.host` empty when the main app ingress is also enabled. A hostless `/` Keycloak ingress can conflict with the hostless app ingress.
- Create a Keycloak DB secret first. This secret must contain the DB password under the key used by `keycloak.db.passwordKey` (defaults to `password`):
  ```bash
  kubectl -n ipam create secret generic keycloak-db \
    --from-literal=password="$POSTGRES_PASSWORD"
  ```
- Create a realm configmap if you want to auto-import the Helm-oriented sample realm (this file matches `ipam.local` and `keycloak.local`, and includes a demo user `devuser` / `devpassword`):
  ```bash
  kubectl -n ipam create configmap ipam-realm --from-file=ipam-realm.json=dev/example-prod-realm.json
  ```
- Create TLS secrets for both ingresses before deploying:
  ```bash
  kubectl -n ipam create secret tls ipam-local-tls \
    --cert=./ipam.local+1.pem \
    --key=./ipam.local+1-key.pem

  kubectl -n ipam create secret tls keycloak-local-tls \
    --cert=./ipam.local+1.pem \
    --key=./ipam.local+1-key.pem
  ```
- Keep `dev/ipam-realm.json` for local development with `make run` or the compose-based Keycloak stack. It is intentionally scoped to localhost-style origins.
- Example deploy with Keycloak enabled:
  ```bash
  helm upgrade --install ipam deploy/helm/ipam -n ipam \
     --set db.existingSecret=ipam-db \
     --set ingress.enabled=true \
     --set ingress.className=nginx \
     --set ingress.hosts[0].host=ipam.local \
     --set ingress.tls[0].secretName=ipam-local-tls \
     --set ingress.tls[0].hosts[0]=ipam.local \
     --set fe.env.VITE_KEYCLOAK_URL=https://keycloak.local \
     --set fe.env.VITE_KEYCLOAK_REALM=ipam \
     --set fe.env.VITE_KEYCLOAK_CLIENT_ID=ipam-fe \
     --set api.auth.enabled=true \
     --set api.auth.issuer=https://keycloak.local/realms/ipam \
     --set api.auth.audience=ipam-api \
     --set api.auth.jwksURL=http://ipam-keycloak:8080/realms/ipam/protocol/openid-connect/certs \
     --set keycloak.enabled=true \
     --set keycloak.db.existingSecret=keycloak-db \
     --set keycloak.hostname.url=https://keycloak.local \
     --set keycloak.ingress.enabled=true \
     --set keycloak.ingress.className=nginx \
     --set keycloak.ingress.tls[0].secretName=keycloak-local-tls \
     --set keycloak.ingress.tls[0].hosts[0]=keycloak.local \
     --set keycloak.ingress.host=keycloak.local \
     --set keycloak.realmImport.enabled=true \
     --set keycloak.realmImport.configMapName=ipam-realm
  ```
- Add host entries for both ingresses:
  ```text
  <minikube-ip> ipam.local keycloak.local
  ```
- Open `https://ipam.local/` to start the browser login flow. Keycloak should be reachable at `https://keycloak.local/`.
- API auth toggle/env:
  - `api.auth.enabled`, `api.auth.issuer`, and `api.auth.audience` must all be set together. When enabled, the API requires a Bearer token for application routes and still skips `/healthz`, `/readyz`, and Swagger.
  - `api.auth.issuer` must match the exact realm issuer URL, for example `https://keycloak.local/realms/ipam`.
  - `api.auth.audience` should match the API audience expected in the token, for example `ipam-api`.
  - `api.auth.jwksURL` should point to the in-cluster Keycloak service when the public issuer host is only resolvable on your workstation, for example `http://ipam-keycloak:8080/realms/ipam/protocol/openid-connect/certs`.
  - The frontend reads its Keycloak runtime config from `env.js`, via Helm `fe.env` (`VITE_KEYCLOAK_URL`, `VITE_KEYCLOAK_REALM`, `VITE_KEYCLOAK_CLIENT_ID`).
