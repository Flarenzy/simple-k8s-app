# Integration Test Context

The integration suite under `integration/api` starts PostgreSQL and Keycloak with the selected container provider, applies migrations, launches the real Go server, and exercises authenticated HTTP journeys. Tests require the integration build tag and a working container runtime.

Extend the customer journey when adding a feature so persistence, authentication, routing, and response contracts are checked together. Run with `make test-integration`.

## Container runtime compatibility

`make test-integration` selects a provider at startup and prints the selection and detected alternatives. On an Apple silicon Mac, a running Apple Container installation is preferred. Elsewhere, or when Apple Container is unavailable and a Docker/Podman endpoint is detected, the existing Testcontainers provider is used. Override automatic selection with `INTEGRATION_CONTAINER_RUNTIME=apple` or `INTEGRATION_CONTAINER_RUNTIME=testcontainers`; `docker` and `podman` are accepted as aliases for the Testcontainers path.

Apple Container consumes OCI images and supports host port publishing and bind mounts, but it does not expose the Docker-compatible API required by Testcontainers. The Apple path is therefore a narrow CLI adapter rather than a Testcontainers socket configuration. It runs the same `postgres:16` and `quay.io/keycloak/keycloak:24.0.5` images, publishes their service ports only on `127.0.0.1`, mounts the Keycloak realm fixture read-only, performs readiness checks from the host, and stops/deletes its uniquely named containers during normal exit, setup failure, SIGINT, and SIGTERM. `--rm` provides an additional cleanup path once a container stops. SIGKILL and host failure cannot run process cleanup; remove a stranded `ipam-it-*` container with `container delete --force <name>`.

The tested Apple workflow requires macOS 26 or later on Apple silicon, the signed [`container` CLI](https://github.com/apple/container#installation), and a running service:

```sh
container system start
container system status
make test-integration
```

The minimal infrastructure proof of concept behind the adapter is:

```sh
set -e
trap 'container stop ipam-postgres-poc >/dev/null 2>&1 || true; container stop ipam-keycloak-poc >/dev/null 2>&1 || true' EXIT
container run -d --rm --name ipam-postgres-poc -p 127.0.0.1:55432:5432 \
  -e POSTGRES_DB=ipam -e POSTGRES_USER=ipam -e POSTGRES_PASSWORD=ipam postgres:16
until container exec ipam-postgres-poc pg_isready -U ipam -d ipam; do sleep 1; done
container run -d --rm --name ipam-keycloak-poc --memory 2g -p 127.0.0.1:58080:8080 \
  -e KEYCLOAK_ADMIN=admin -e KEYCLOAK_ADMIN_PASSWORD=admin \
  --mount "type=bind,source=$(pwd)/integration/api/testdata,target=/opt/keycloak/data/import,readonly" \
  quay.io/keycloak/keycloak:24.0.5 start-dev --http-port=8080 --import-realm
until curl --fail http://127.0.0.1:58080/realms/ipam-integration/.well-known/openid-configuration; do sleep 1; done
```

The real adapter allocates free host ports instead of using the fixed proof-of-concept ports. PostgreSQL readiness uses an authenticated database ping, and Keycloak readiness uses the realm discovery endpoint; an Apple port-forward listener alone is not treated as service readiness. The Go API and tests run on the host and connect through loopback, so no container-to-container or container-to-host DNS setup is required.

Apple Container pulls standard OCI images automatically. Use `container registry login <registry>` before the test when an image override needs authentication. Both default test images provide native Linux arm64 variants, so the adapter uses the host architecture and does not require Rosetta. An override that only provides amd64 is unsupported by this workflow; prefer a multi-platform image with an arm64 variant.

Configuration:

| Variable | Default | Purpose |
| --- | --- | --- |
| `INTEGRATION_CONTAINER_RUNTIME` | `auto` | `apple`, `testcontainers`, or the `docker`/`podman` aliases |
| `INTEGRATION_CONTAINER_STARTUP_TIMEOUT` | `2m` | Image pull, PostgreSQL, and Keycloak startup/readiness timeout as a Go duration |
| `INTEGRATION_POSTGRES_IMAGE` | `postgres:16` | OCI image override |
| `INTEGRATION_KEYCLOAK_IMAGE` | `quay.io/keycloak/keycloak:24.0.5` | OCI image override |

The Docker/Podman path remains the CI-compatible default on non-macOS hosts and retains Testcontainers host discovery, including `DOCKER_HOST` and `~/.testcontainers.properties`. A missing/stopped runtime, failed image pull, or readiness timeout fails the suite rather than skipping it and reports the selected runtime, detected alternatives, corrective command, and container output where available.

The compatibility decision is based on the authoritative [Apple Container overview and requirements](https://github.com/apple/container), [Apple networking, mount, architecture, and registry guidance](https://github.com/apple/container/blob/main/docs/how-to.md), [Apple command reference](https://github.com/apple/container/blob/main/docs/command-reference.md), and [Testcontainers Docker API requirement](https://golang.testcontainers.org/system_requirements/docker/).
