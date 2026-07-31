# Backend: Sites and Usage Aggregates

## Status

The first sites rollout is implemented and validated locally. Sites and site-level statistics are wired through migrations, SQLC, repositories, domain services, HTTP handlers, frontend flows, Swagger, unit tests, and API integration coverage. The container-backed integration run remains environment-dependent on Docker.

## Goal

Add backend support for:

- site entities
- subnet-to-site relationships
- subnet usage metrics
- site-level aggregated IP usage metrics

This task provides the API and data model needed for the planned frontend refactor and dashboard work.

## Current State

- The backend currently supports subnets and IP addresses only.
- There is no `sites` table.
- `subnets` does not currently reference a site.
- The subnet list API does not return usage metrics.
- There is no API that returns site-level aggregate usage data.

## Required Product Direction

- Sites map to subnets, not IP addresses.
- A site can own zero or more subnets.
- The preferred long-term hierarchy is `site -> subnets (CIDRs) -> IPs`.
- Site-level usage is the aggregate of all subnets assigned to that site.
- The backend must return aggregate usage information; the frontend should not be the primary source of truth for these calculations.
- The first rollout must be migration-safe for existing data.
- Site deletion should be allowed for now and should clear `subnets.site_id`.
- New subnet creation must require a site immediately, even while legacy rows may still have `NULL site_id`.
- `total_ips` must represent usable host capacity, not raw CIDR size.
- Site names must be unique.
- Subnet-to-site assignment should be modeled as a subnet mutation.
- Preferred first-rollout endpoint: `PATCH /api/v1/subnets/{id}/site`.
- Direct API clearing of a subnet's `site_id` is not supported in the first rollout.
- The only supported clearing behavior in the first rollout is indirect clearing caused by site deletion.

Target state:

- new subnets should be created under a site
- a subnet should not be considered a top-level standalone resource in the long-term model
- existing data may require a migration-friendly transitional period before enforcing that every subnet has a site

## Planned Work

### Database Schema

- [x] Add a new `sites` table with fields:
  - `id`
  - `name`
  - `description`
  - `created_at`
  - `updated_at`
- [x] Add a `site_id` relationship to `subnets`, with a migration strategy that can support existing rows before the model is fully enforced.
- [x] Add a foreign key from `subnets.site_id` to `sites.id`.
- [x] Decide and implement the deletion rule for sites carefully:
  - first-rollout rule: `ON DELETE SET NULL`
  - long-term required rule: prevent deleting a site while subnets are still attached, and require reassignment or subnet removal first
- [x] Add a unique constraint for `sites.name`.
- [ ] Add any required indexes for:
  - looking up subnets by site
  - listing sites with aggregate counts efficiently

### Domain Model

- [ ] Add a `Site` domain model.
- [ ] Extend the `Subnet` domain model to include `SiteID` and, if useful for response construction, `SiteName`.
- [ ] Define input models for:
  - creating a site
  - updating a site
  - assigning a subnet's site

### Repository Layer

- [x] Add repository support for site CRUD.
- [x] Add repository support for assigning `site_id` on subnets.
- [ ] Preserve migration-safe support for legacy `NULL site_id` rows that predate the new flow.
- [ ] Add queries for subnet usage metrics:
  - count persisted IP rows per subnet
  - derive usable-host capacity from CIDR in service/domain logic
- [x] Add queries for site-level aggregate metrics:
  - `subnet_count`
  - `used_ips`
  - `total_ips`
  - optionally `free_ips` or enough data for it to be derived safely

### Service Layer

- [x] Add service methods for site CRUD.
- [x] Add service methods for assigning a subnet to a site.
- [ ] Do not expose a normal service path for clearing a subnet's site outside site deletion in the first rollout.
- [ ] Extend subnet listing/get methods so they can return usage metrics.
- [ ] Add service logic to compute usable-host `total_ips` from each subnet CIDR.
- [ ] Add service logic to aggregate site usage across all attached subnets.

### HTTP API

- [x] Add site endpoints:
  - `GET /api/v1/sites`
  - `POST /api/v1/sites`
  - `PATCH /api/v1/sites/{id}`
  - `DELETE /api/v1/sites/{id}`
- [ ] Ensure `GET /api/v1/sites` returns backend-computed aggregate fields:
  - `subnet_count`
  - `used_ips`
  - `total_ips`
  - optionally `free_ips`
- [ ] Extend subnet responses to include:
  - `used_ips`
  - `total_ips`
  - `site_id`
  - optionally `site_name`
- [ ] Add subnet site-assignment behavior through:
  - `PATCH /api/v1/subnets/{id}/site`
  - request body contains required `site_id`
  - direct clearing via `site_id: null` is not supported in the first rollout
- [x] Add or extend subnet creation behavior so new subnets can be created under a site as part of the normal flow.
- [ ] Require `site_id` in new subnet creation requests.

The first rollout keeps `site_id` optional for migration compatibility; the frontend supports assigning it during subnet creation and editing.

### CSV Import Follow-Up

- [ ] Keep a future bulk import feature in mind while designing schema and APIs.
- [ ] Plan for a follow-up CSV import flow using columns such as:
  - `site`
  - `cidr`
  - `ip`
  - `description`
- [ ] The future import flow should be able to:
  - create a site if it does not exist
  - create a subnet under that site if it does not exist
  - create or update IP metadata for the specified address
- [ ] This CSV import is a follow-up feature and should not be implemented as part of the initial sites/aggregates task.

## Suggested API Shapes

### Site response

- `id`
- `name`
- `description`
- `subnet_count`
- `used_ips`
- `total_ips`
- optionally `free_ips`
- `created_at`
- `updated_at`

### Extended subnet response

- existing subnet fields
- `used_ips`
- `total_ips`
- `site_id`
- optionally `site_name`

### Subnet site assignment input

- use a dedicated endpoint:
  - `PATCH /api/v1/subnets/{id}/site`
  - request body includes required `site_id`
  - direct clearing is not a normal supported operation in the first rollout

The endpoint shape is chosen for the first rollout, and site assignment remains a subnet-level concern.

## Aggregate Rules

- `used_ips` for a subnet = count of persisted `ip_addresses` rows for that subnet.
- `total_ips` for a subnet = usable host capacity derived from the subnet CIDR.
- `free_ips` for a subnet = `total_ips - used_ips`.
- `used_ips` for a site = sum of `used_ips` across all subnets assigned to that site.
- `total_ips` for a site = sum of `total_ips` across all subnets assigned to that site.
- `free_ips` for a site = `total_ips - used_ips`.

These rules should be implemented consistently in the service layer, even if some fields are materialized in SQL queries for efficiency.

## Validation

- [x] Migrations apply cleanly.
- [x] Site CRUD works as expected.
- [ ] Deleting a site leaves subnets intact and clears their `site_id`.
- [ ] New subnet creation rejects requests without `site_id`.
- [ ] Subnets can be assigned to a site.
- [ ] Direct API clearing of a subnet's `site_id` is rejected in the first rollout.
- [ ] Subnet list responses include `used_ips` and `total_ips`.
- [x] Site list responses include aggregate usage metrics.
- [ ] Aggregate values are correct for:
  - sites with no subnets
  - sites with one subnet
  - sites with multiple subnets
  - subnets with zero saved IPs
  - subnets with saved IPs

## Testing

- [ ] Follow strict TDD for each vertical slice:
  - write boundary tests first
  - implement the minimum code to pass
  - add integration coverage for schema and wiring before broad refactors
- [ ] Require tests for all new behavior:
  - unit tests for domain/service logic
  - fuzz tests for any new parsing or normalization code
  - integration tests for schema, persistence, and end-to-end API contracts
- [ ] Add repository tests for site CRUD and subnet-site assignment behavior.
- [x] Add service tests for:
  - subnet usable-host capacity calculation
  - site aggregate calculation
  - null-site behavior
- [x] Add HTTP handler tests for:
  - site CRUD endpoints
  - subnet responses with usage fields
  - site responses with aggregate fields
- [x] Update integration tests where needed to reflect the new response shapes.

## Recommended TDD Slice Order

- [ ] Slice 1: schema and migration rules
  - `sites` table exists
  - `subnets.site_id` exists and supports legacy `NULL` rows
  - deleting a site clears `site_id`
  - unique site names are enforced
- [ ] Slice 2: `GET /api/v1/sites`
  - empty list returns `[]`
  - site with no subnets returns zeroed aggregate fields
  - one site with one subnet returns correct aggregate fields
- [ ] Slice 3: aggregate domain math
  - usable hosts for representative CIDRs (`/24`, `/31`, IPv6 cases)
  - site aggregate sums across zero, one, and many subnets
- [ ] Slice 4: subnet creation with required `site_id`
  - missing `site_id` returns `400`
  - unknown `site_id` returns the chosen not-found contract
  - success returns `site_id`, `used_ips`, and `total_ips`
- [ ] Slice 5: subnet site assignment
  - `PATCH /api/v1/subnets/{id}/site` assigns a valid site
  - invalid IDs return client errors
  - unknown site returns not-found
  - direct clearing is rejected
- [ ] Slice 6: full integration pass
  - migrations
  - persistence
  - updated response contracts

## Ownership

- Backend implementation will be done by you.
- A coding agent may review backend changes as you build them.
- This task is the backend counterpart to the separate frontend refactor task, which should be implemented by a coding agent.

## Notes

- This task should stay focused on backend data model, business logic, and API contracts.
- The intended long-term ownership chain is `site -> subnets -> IPs`.
- Once the site-owned subnet model is enforced, site deletion should be blocked when subnets are still attached.
- CSV import should be considered when shaping the API so future bulk ingestion does not require a redesign.
- Frontend rendering and UI changes belong in the separate planned task:
  - `tasks/planned/frontend-refactor-dashboard-sites.md`
