# Deployment Context

`deploy/docker` contains separate images for the API, frontend, and database migration job. `deploy/helm` contains the Kubernetes packaging for the API, frontend, migrations, PostgreSQL dependency, optional Keycloak, and ingress or Gateway API exposure.

Deployment configuration passes database, authentication, CORS, and frontend runtime settings through environment variables. `values-kiac.yaml` is the local Gateway API + Keycloak profile for first installation; `make kiac-deploy` updates an existing local release from the latest merged image SHA while preserving its Gateway and HTTPS values, creates any missing local Keycloak Secrets, and owns the local-only discovery defaults in `deploy/kiac-update.sh`. Validate chart changes with Helm rendering and keep image/service names consistent across templates and values.
