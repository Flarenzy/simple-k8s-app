-- db/migrations/0002_create_subnets.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS ip_addresses 
(
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip         INET NOT NULL,
    hostname   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    subnet_id  BIGINT NOT NULL,
    CONSTRAINT fk_subnetid FOREIGN KEY (subnet_id) REFERENCES subnets(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE ip_addresses;
