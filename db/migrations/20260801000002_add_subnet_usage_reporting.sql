-- +goose Up
CREATE TABLE reporting_settings (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    cadence TEXT NOT NULL DEFAULT 'hourly' CHECK (cadence IN ('hourly', 'daily', 'weekly')),
    retention_days INTEGER NOT NULL DEFAULT 30 CHECK (retention_days BETWEEN 1 AND 180),
    last_snapshot_at TIMESTAMPTZ
);

INSERT INTO reporting_settings (singleton) VALUES (TRUE);

CREATE TABLE subnet_usage_snapshots (
    subnet_id BIGINT NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    captured_at TIMESTAMPTZ NOT NULL,
    used_ips BIGINT NOT NULL CHECK (used_ips >= 0),
    total_ips BIGINT NOT NULL CHECK (total_ips >= used_ips),
    PRIMARY KEY (subnet_id, captured_at)
);

CREATE INDEX subnet_usage_snapshots_captured_at_idx
    ON subnet_usage_snapshots (captured_at);

-- +goose Down
DROP TABLE subnet_usage_snapshots;
DROP TABLE reporting_settings;
