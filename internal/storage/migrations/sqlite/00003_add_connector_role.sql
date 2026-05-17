-- +goose Up
ALTER TABLE connectors ADD COLUMN role TEXT NULL;

-- +goose Down
ALTER TABLE connectors DROP COLUMN role;
