-- name: ListSubnets :many
SELECT id, cidr, description, created_at, updated_at
FROM subnets
ORDER BY id;

-- name: CreateSubnet :one
INSERT INTO subnets (cidr, description)
VALUES ($1, $2)
RETURNING id, cidr, description, created_at, updated_at;

-- name: GetSubnetByID :one
SELECT id, cidr, description, created_at, updated_at
FROM subnets
WHERE id = $1;
