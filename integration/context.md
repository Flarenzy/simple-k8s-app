# Integration Test Context

The integration suite under `integration/api` starts PostgreSQL and Keycloak with Testcontainers, applies migrations, launches the real Go server, and exercises authenticated HTTP journeys. Tests require the integration build tag and a working container runtime.

Extend the customer journey when adding a feature so persistence, authentication, routing, and response contracts are checked together. Run with `make test-integration`.
