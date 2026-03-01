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
- Tagged releases also publish a signed OCI Helm chart to `oci://ghcr.io/flarenzy/charts/ipam`. The matching `.tgz`, `.prov`, and public signing key are attached to the GitHub Release so consumers can verify provenance. The public key is committed at `docs/helm-release-public.asc`.

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
