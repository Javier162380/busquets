-- +goose Up
ALTER TABLE plans ADD COLUMN sync_source TEXT NOT NULL DEFAULT '';
ALTER TABLE plans DROP CONSTRAINT plans_file_name_key;
ALTER TABLE plans ADD CONSTRAINT plans_file_name_sync_source_key UNIQUE(file_name, sync_source);

-- +goose Down
ALTER TABLE plans DROP CONSTRAINT plans_file_name_sync_source_key;
ALTER TABLE plans ADD CONSTRAINT plans_file_name_key UNIQUE(file_name);
ALTER TABLE plans DROP COLUMN sync_source;
