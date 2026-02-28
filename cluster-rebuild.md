# Cluster Rebuild

## 1. Update and verify `minikube`

```bash
brew update
brew upgrade minikube
minikube version
brew list --versions minikube
```

## 2. Recreate the cluster

Use the default supported Kubernetes version:

```bash
minikube delete
minikube start --driver=podman
```

If you want the newer version explicitly:

```bash
minikube delete
minikube start --driver=podman --kubernetes-version=v1.35.1
```

## 3. Enable ingress and keep the tunnel running

```bash
minikube addons enable ingress
minikube tunnel
```

Run `minikube tunnel` in its own terminal and leave it running.

## 4. Get the Minikube IP

```bash
minikube ip
```

## 5. Add local host entries

Replace `<minikube-ip>` with the output from `minikube ip`.

```text
<minikube-ip> ipam.local keycloak.local
```

## 6. Install Postgres

```bash
export POSTGRES_PASSWORD="yourpassword"

helm upgrade --install ipam-postgres bitnami/postgresql \
  -n ipam --create-namespace \
  --set auth.username=ipam \
  --set auth.password=$POSTGRES_PASSWORD \
  --set auth.database=ipam
```

## 7. Create app and Keycloak secrets

```bash
kubectl -n ipam create secret generic ipam-db \
  --from-literal=DB_CONN="postgres://ipam:${POSTGRES_PASSWORD}@ipam-postgres-postgresql.ipam.svc.cluster.local:5432/ipam?sslmode=disable"

kubectl -n ipam create secret generic keycloak-db \
  --from-literal=password="$POSTGRES_PASSWORD"
```

If the secrets already exist and you are re-running this, delete and recreate them:

```bash
kubectl -n ipam delete secret ipam-db keycloak-db
```

Then run the create commands again.

## 8. Create local TLS secrets

```bash
kubectl -n ipam create secret tls ipam-local-tls \
  --cert=./ipam.local+1.pem \
  --key=./ipam.local+1-key.pem

kubectl -n ipam create secret tls keycloak-local-tls \
  --cert=./ipam.local+1.pem \
  --key=./ipam.local+1-key.pem
```

If they already exist:

```bash
kubectl -n ipam delete secret ipam-local-tls keycloak-local-tls
```

Then run the create commands again.

## 9. Create the Keycloak realm configmap

```bash
kubectl -n ipam create configmap ipam-realm \
  --from-file=ipam-realm.json=dev/example-prod-realm.json
```

If it already exists:

```bash
kubectl -n ipam delete configmap ipam-realm
```

Then run the create command again.

## 10. Deploy the app and Keycloak

```bash
helm upgrade --install ipam deploy/helm/ipam \
  -n ipam \
  -f values-keycloak.yaml
```

## 11. Verify rollout

```bash
kubectl -n ipam get pods
kubectl -n ipam get ingress
kubectl -n ipam get jobs
```

## 12. Open the app

```text
https://ipam.local/
https://keycloak.local/
```

Sample realm credentials:

```text
username: devuser
password: devpassword
```
