# HTTP API Context

`api.go` builds the `net/http` router and middleware stack. `handlers.go` translates requests into domain service calls, `models.go` defines JSON request/response shapes, and the auth/CORS middleware wraps the routes.

The API exposes health/readiness, Swagger, subnet CRUD, IP operations, and site CRUD/statistics under `/api/v1`. Site endpoints are `GET/POST /api/v1/sites`, `GET /api/v1/sites/statistics`, `GET/PATCH/DELETE /api/v1/sites/{id}`. Site names must contain non-whitespace characters; invalid site payloads return `400` before reaching the service. Site statistics aggregate subnets associated through `site_id`, count used IPs, and report safely representable address capacity.
