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

## Optional: Keycloak

- The supported Minikube + Keycloak topology uses two explicit hosts:
  - `ipam.local` for the frontend and API
  - `keycloak.local` for the Keycloak ingress
- Do not leave `keycloak.ingress.host` empty when the main app ingress is also enabled. A hostless `/` Keycloak ingress can conflict with the hostless app ingress.
- Create a Keycloak DB secret first. This secret must contain the DB password under the key used by `keycloak.db.passwordKey` (defaults to `password`):
  ```bash
  kubectl -n ipam create secret generic keycloak-db \
    --from-literal=password="$POSTGRES_PASSWORD"
  ```
- Create a realm configmap if you want to auto-import a sample realm:
  ```bash
  kubectl -n ipam create configmap ipam-realm --from-file=ipam-realm.json=dev/ipam-realm.json
  ```
- Example deploy with Keycloak enabled:
  ```bash
  helm upgrade --install ipam deploy/helm/ipam -n ipam \
     --set db.existingSecret=ipam-db \
     --set ingress.enabled=true \
     --set ingress.className=nginx \
     --set ingress.hosts[0].host=ipam.local \
     --set fe.env.VITE_KEYCLOAK_URL=http://keycloak.local \
     --set fe.env.VITE_KEYCLOAK_REALM=ipam \
     --set fe.env.VITE_KEYCLOAK_CLIENT_ID=ipam-fe \
     --set api.auth.enabled=true \
     --set api.auth.issuer=http://keycloak.local/realms/ipam \
     --set api.auth.audience=ipam-api \
     --set keycloak.enabled=true \
     --set keycloak.db.existingSecret=keycloak-db \
     --set keycloak.hostname.url=http://keycloak.local \
     --set keycloak.ingress.enabled=true \
     --set keycloak.ingress.className=nginx \
     --set keycloak.ingress.host=keycloak.local \
     --set keycloak.realmImport.enabled=true \
     --set keycloak.realmImport.configMapName=ipam-realm
  ```
- Add host entries for both ingresses:
  ```text
  <minikube-ip> ipam.local keycloak.local
  ```
- Open `http://ipam.local/` to start the browser login flow. Keycloak should be reachable at `http://keycloak.local/`.
- API auth toggle/env:
  - `api.auth.enabled`, `api.auth.issuer`, and `api.auth.audience` must all be set together. When enabled, the API requires a Bearer token for application routes and still skips `/healthz`, `/readyz`, and Swagger.
  - `api.auth.issuer` must match the exact realm issuer URL, for example `http://keycloak.local/realms/ipam`.
  - `api.auth.audience` should match the API audience expected in the token, for example `ipam-api`.
  - The frontend reads its Keycloak runtime config from `env.js`, via Helm `fe.env` (`VITE_KEYCLOAK_URL`, `VITE_KEYCLOAK_REALM`, `VITE_KEYCLOAK_CLIENT_ID`).
