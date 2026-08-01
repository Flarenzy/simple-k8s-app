-- name: GetReportingSettings :one
SELECT cadence, retention_days, last_snapshot_at
FROM reporting_settings
WHERE singleton = TRUE;

-- name: UpdateReportingSettings :one
UPDATE reporting_settings
SET cadence = $1, retention_days = $2
WHERE singleton = TRUE
RETURNING cadence, retention_days, last_snapshot_at;

-- name: CaptureDueSubnetUsageSnapshots :one
WITH due AS (
    UPDATE reporting_settings
    SET last_snapshot_at = NOW()
    WHERE singleton = TRUE
      AND (
          last_snapshot_at IS NULL
          OR last_snapshot_at <= NOW() - CASE cadence
              WHEN 'hourly' THEN INTERVAL '1 hour'
              WHEN 'daily' THEN INTERVAL '1 day'
              WHEN 'weekly' THEN INTERVAL '1 week'
          END
      )
    RETURNING last_snapshot_at
), inserted AS (
    INSERT INTO subnet_usage_snapshots (subnet_id, captured_at, used_ips, total_ips)
    SELECT subnets.id,
           due.last_snapshot_at,
           COUNT(ip_addresses.id),
           GREATEST(
               COUNT(ip_addresses.id),
               POWER(2::numeric, (32 - masklen(subnets.cidr))::numeric)::bigint
           )
    FROM subnets
    CROSS JOIN due
    LEFT JOIN ip_addresses ON ip_addresses.subnet_id = subnets.id
    WHERE family(subnets.cidr) = 4
    GROUP BY subnets.id, subnets.cidr, due.last_snapshot_at
    RETURNING 1
)
SELECT COUNT(*) FROM inserted;

-- name: DeleteExpiredSubnetUsageSnapshots :execrows
DELETE FROM subnet_usage_snapshots
WHERE captured_at < NOW() - make_interval(days => (
    SELECT retention_days FROM reporting_settings WHERE singleton = TRUE
));

-- name: ListSubnetUsageSnapshots :many
SELECT subnet_id, captured_at, used_ips, total_ips
FROM subnet_usage_snapshots
WHERE subnet_id = $1
  AND captured_at >= $2
  AND captured_at <= $3
ORDER BY captured_at;
