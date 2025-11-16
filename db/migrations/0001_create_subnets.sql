-- db/migrations/0001_create_subnets.sql

-- +goose Up
CREATE TABLE subnets (
    id          bigserial PRIMARY KEY,
    cidr        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subnets;
