-- +goose Up
-- +goose StatementBegin
ALTER TABLE subnets ADD COLUMN site_id uuid;
ALTER TABLE subnets ADD CONSTRAINT fk_subnet_to_site FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE subnets DROP CONSTRAINT fk_subnet_to_site;
ALTER TABLE subnets DROP COLUMN site_id;
-- +goose StatementEnd
