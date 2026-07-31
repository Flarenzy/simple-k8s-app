-- name: ListSubnets :many
SELECT subnets.id, subnets.cidr, subnets.description, subnets.created_at, subnets.updated_at, subnets.site_id,
       (SELECT COUNT(*) FROM ip_addresses WHERE subnet_id = subnets.id) AS used_ips
FROM subnets
ORDER BY subnets.id;

-- name: CreateSubnet :one
INSERT INTO subnets (cidr, site_id, description)
VALUES ($1, $2, $3)
RETURNING id, cidr, description, created_at, updated_at, site_id;

-- name: GetSubnetByID :one
SELECT subnets.id, subnets.cidr, subnets.description, subnets.created_at, subnets.updated_at, subnets.site_id,
       (SELECT COUNT(*) FROM ip_addresses WHERE subnet_id = subnets.id) AS used_ips
FROM subnets
WHERE subnets.id = $1;

-- name: UpdateSubnet :one
UPDATE subnets
SET cidr = $2, site_id = $3, description = $4, updated_at = now() AT TIME ZONE 'UTC'
WHERE id = $1
RETURNING id, cidr, description, created_at, updated_at, site_id;

-- name: AssignSubnetSite :one
UPDATE subnets
SET site_id = $2, updated_at = now() AT TIME ZONE 'UTC'
WHERE id = $1
RETURNING id, cidr, description, created_at, updated_at, site_id;

-- name: DeleteSubnetByID :one
WITH deleted_rows AS (
    DELETE FROM subnets
    WHERE id = $1
    RETURNING *
)
SELECT count(*) FROM deleted_rows;
