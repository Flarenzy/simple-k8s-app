# Frontend Context

The frontend is a Vite-powered React and TypeScript application. `src/main.tsx` mounts `App.tsx`, which currently owns the subnet list, subnet detail, IP table, Kubernetes Service observation panels, API requests, and user-facing loading/error states. `src/keycloak.ts` provides optional Keycloak login and token refresh; `src/authz.js` maps `ipam-api` client roles to UI capabilities; `src/env.ts` reads runtime or build-time configuration.

The API base defaults to `/api/v1` and can be set with `VITE_API_BASE`. Build with `npm run build` and run focused tests with `npm test` from this directory. Keep API response types aligned with the backend JSON models as new site views are added. Validate authenticated behavior with a browser smoke test covering role-specific controls, site list/create/edit/delete, statistics loading, and subnet-detail Service observations and per-address match states.
