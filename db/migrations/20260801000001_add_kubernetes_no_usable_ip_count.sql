-- +goose Up
ALTER TABLE kubernetes_sources
ADD COLUMN no_usable_ip_count INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE kubernetes_sources
DROP COLUMN no_usable_ip_count;
