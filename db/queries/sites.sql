-- name: ListSites :many
SELECT id, name, description, created_at, updated_at
FROM sites
ORDER BY id;

-- name: CreateSite :one
INSERT INTO sites (id, name, description)
VALUES ($1, $2, $3)
RETURNING id, name , description, created_at, updated_at;

-- name: GetSiteByID :one
SELECT id, name, description, created_at, updated_at
FROM sites
WHERE id = $1;

-- name: DeleteSiteByID :one
WITH deleted_rows AS (
    DELETE FROM sites
        WHERE id = $1
        RETURNING *
)
SELECT count(*) FROM deleted_rows;


-- name: PerSubnetStatistics :many
SELECT sites.id, subnets.id sub_id, subnets.cidr as cidr, COUNT(ip_addresses.id) as used_ips
FROM sites
LEFT JOIN subnets
ON sites.id = subnets.site_id
LEFT JOIN ip_addresses
ON subnets.id = ip_addresses.subnet_id
GROUP BY sites.id, subnets.id, subnets.cidr;

-- name: UpdateSite :one
UPDATE sites
SET name = $2, description = $3, updated_at = now() AT TIME ZONE 'UTC'
WHERE id = $1
RETURNING *;
