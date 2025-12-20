-- name: ListIPsBySubnetID :many
SELECT id, ip, hostname, created_at, updated_at, subnet_id
FROM ip_addresses
WHERE subnet_id = $1
ORDER by ip;

-- name: CreateIPAddress :one
INSERT INTO ip_addresses (ip, hostname, subnet_id)
VALUES ($1, $2, $3)
RETURNING id, ip, hostname, created_at, updated_at, subnet_id;

-- name: UpdateIPByUUID :one
UPDATE ip_addresses
SET hostname = $1, updated_at = NOW()
WHERE id = $2
RETURNING id, ip, hostname, created_at, updated_at, subnet_id;
