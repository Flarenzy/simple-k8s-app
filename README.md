# Simple IPAM application
The point of this repo is to learn on how to deploy a application on a minikube cluster.
The idea is to have a Kubernetes ingress, a NGINX or similar proxy for statically serviring the frontend, golang API for all CRUD operations, Keycloak for authorization and authentification and Postgres as a persistant data store.
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

3. Deploy the app chart (ingress enabled by default, host left empty so the minikube IP works):
   ```bash
   helm upgrade --install ipam deploy/helm/ipam -n ipam \
     --set db.existingSecret=ipam-db \
     --set ingress.enabled=true \
     --set ingress.className=nginx
   ```

4. Access the app: with the tunnel running, open `http://<minikube-ip>/` (FE) and `http://<minikube-ip>/api/v1/healthz` (API). If you prefer a host, set `ingress.hosts[0].host` and add it to `/etc/hosts`.

Notes:
- Migration hook runs as a Helm post-install/upgrade job using the `ipam-migrate` image; ensure `DB_CONN` secret exists before deploying.
- Images are published to GHCR: `ghcr.io/flarenzy/ipam-api`, `ghcr.io/flarenzy/ipam-fe`, `ghcr.io/flarenzy/ipam-migrate`.
