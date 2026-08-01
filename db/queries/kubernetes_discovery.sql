-- name: TryKubernetesSourceLock :one
SELECT pg_try_advisory_xact_lock(hashtextextended($1, 0));

-- name: UpsertKubernetesSource :one
INSERT INTO kubernetes_sources (
    source_key, name, site_id, cluster_domain, namespace_scope, updated_at
) VALUES ($1, $2, $3, $4, $5::text[], now())
ON CONFLICT (source_key) DO UPDATE SET
    name = EXCLUDED.name,
    site_id = EXCLUDED.site_id,
    cluster_domain = EXCLUDED.cluster_domain,
    namespace_scope = EXCLUDED.namespace_scope,
    updated_at = now()
RETURNING *;

-- name: EnsureKubernetesSource :exec
INSERT INTO kubernetes_sources (
    source_key, name, site_id, cluster_domain, namespace_scope
) VALUES ($1, $2, $3, $4, $5::text[])
ON CONFLICT (source_key) DO NOTHING;

-- name: GetKubernetesSourceByKey :one
SELECT * FROM kubernetes_sources WHERE source_key = $1;

-- name: UpsertKubernetesService :one
INSERT INTO kubernetes_services (
    source_id, kubernetes_uid, namespace, name, service_type,
    resource_version, external_name, dns_name, observed_at, active, stale_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, true, NULL)
ON CONFLICT (source_id, kubernetes_uid) DO UPDATE SET
    namespace = EXCLUDED.namespace,
    name = EXCLUDED.name,
    service_type = EXCLUDED.service_type,
    resource_version = EXCLUDED.resource_version,
    external_name = EXCLUDED.external_name,
    dns_name = EXCLUDED.dns_name,
    observed_at = EXCLUDED.observed_at,
    active = true,
    stale_at = NULL,
    updated_at = now()
RETURNING *;

-- name: DeleteKubernetesServicePorts :exec
DELETE FROM kubernetes_service_ports WHERE service_id = $1;

-- name: CreateKubernetesServicePort :exec
INSERT INTO kubernetes_service_ports (
    service_id, name, protocol, port, target_port, app_protocol, node_port
) VALUES ($1, NULLIF($2, ''), $3, $4, $5, NULLIF($6, ''), NULLIF($7, 0));

-- name: DeleteKubernetesServiceAddresses :exec
DELETE FROM kubernetes_service_addresses WHERE service_id = $1;

-- name: FindIPCandidatesBySiteAndAddress :many
SELECT ip_addresses.id
FROM ip_addresses
JOIN subnets ON subnets.id = ip_addresses.subnet_id
WHERE subnets.site_id = $1 AND ip_addresses.ip = $2::inet
ORDER BY ip_addresses.id;

-- name: CreateKubernetesServiceAddress :exec
INSERT INTO kubernetes_service_addresses (
    service_id, kind, address, ip_mode, ip_address_id, match_status, match_count
) VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7);

-- name: DeleteKubernetesServiceHostnames :exec
DELETE FROM kubernetes_service_hostnames WHERE service_id = $1;

-- name: CreateKubernetesServiceHostname :exec
INSERT INTO kubernetes_service_hostnames (service_id, kind, hostname)
VALUES ($1, $2, $3);

-- name: MarkMissingKubernetesServicesInactive :exec
UPDATE kubernetes_services
SET active = false, stale_at = $3, updated_at = now()
WHERE source_id = $1
  AND active = true
  AND NOT (kubernetes_uid = ANY($2::text[]));

-- name: DeleteStaleKubernetesServices :exec
DELETE FROM kubernetes_services
WHERE source_id = $1 AND active = false AND stale_at <= $2;

-- name: RecordKubernetesSourceSuccess :exec
UPDATE kubernetes_sources
SET last_attempt_at = $2,
    last_success_at = $2,
    last_error = '',
    service_count = $3,
    matched_count = $4,
    unmatched_count = $5,
    ambiguous_count = $6,
    updated_at = now()
WHERE id = $1;

-- name: RecordKubernetesSourceFailure :exec
UPDATE kubernetes_sources
SET last_attempt_at = $2, last_error = $3, updated_at = now()
WHERE id = $1;

-- name: ListKubernetesSourceStatuses :many
SELECT source_key, name, site_id, cluster_domain, namespace_scope,
       last_attempt_at, last_success_at, last_error,
       service_count, matched_count, unmatched_count, ambiguous_count
FROM kubernetes_sources
ORDER BY source_key;

-- name: ListMatchedKubernetesServicesBySubnet :many
SELECT a.ip_address_id,
       src.source_key,
       src.name AS source_name,
       svc.kubernetes_uid,
       svc.name,
       svc.namespace,
       svc.service_type,
       svc.dns_name,
       svc.observed_at,
       a.address,
       a.kind AS address_kind,
       port.name AS port_name,
       port.protocol,
       port.port,
       port.target_port,
       port.app_protocol,
       port.node_port
FROM kubernetes_service_addresses a
JOIN kubernetes_services svc ON svc.id = a.service_id
JOIN kubernetes_sources src ON src.id = svc.source_id
JOIN ip_addresses ip ON ip.id = a.ip_address_id
LEFT JOIN kubernetes_service_ports port ON port.service_id = svc.id
WHERE ip.subnet_id = $1
  AND svc.active = true
  AND a.match_status = 'matched'
ORDER BY a.ip_address_id, src.source_key, svc.namespace, svc.name, svc.kubernetes_uid,
         a.kind, port.port, port.protocol, port.name;
