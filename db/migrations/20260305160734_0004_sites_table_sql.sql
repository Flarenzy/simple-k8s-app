-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sites (
    id          uuid        PRIMARY KEY,
    name        TEXT        UNIQUE NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
    );
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE sites;
-- +goose StatementEnd
