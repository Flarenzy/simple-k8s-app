-- +goose Up
-- +goose StatementBegin
ALTER TABLE ip_addresses 
ADD CONSTRAINT unique_ip UNIQUE (ip, subnet_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE ip_addresses
DROP CONSTRAINT unique_ip;
-- +goose StatementEnd
