-- name: ListSubnets :many
SELECT id, cidr, description, created_at, updated_at, site_id
FROM subnets
ORDER BY id;

-- name: CreateSubnet :one
INSERT INTO subnets (cidr, site_id, description)
VALUES ($1, $2, $3)
RETURNING id, cidr, description, created_at, updated_at, site_id;

-- name: GetSubnetByID :one
SELECT id, cidr, description, created_at, updated_at, site_id
FROM subnets
WHERE id = $1;

-- name: DeleteSubnetByID :one
WITH deleted_rows AS (
    DELETE FROM subnets
    WHERE id = $1
    RETURNING *
)
SELECT count(*) FROM deleted_rows;
