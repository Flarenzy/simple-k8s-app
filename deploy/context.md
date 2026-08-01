# Deployment Context

`deploy/docker` contains separate images for the API, frontend, and database migration job. `deploy/helm` contains the Kubernetes packaging for the API, frontend, migrations, PostgreSQL dependency, optional Keycloak, and ingress or Gateway API exposure.

Deployment configuration passes database, authentication, CORS, and frontend runtime settings through environment variables. `values-kiac.yaml` is the local Gateway API + Keycloak profile; it expects separate Kubernetes Secrets for the Keycloak master bootstrap account and the IPAM application `admin` password. Validate chart changes with Helm rendering and keep image/service names consistent across templates and values.
