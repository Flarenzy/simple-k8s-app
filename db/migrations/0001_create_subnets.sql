-- db/migrations/0001_create_subnets.sql

-- +goose Up
CREATE TABLE IF NOT EXISTS subnets (
    id          BIGSERIAL PRIMARY KEY,
    cidr        CIDR        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subnets;
