-- +goose Up
ALTER TABLE plan_versions ADD COLUMN milestones TEXT; -- JSON array; NULL = none

-- +goose Down
ALTER TABLE plan_versions DROP COLUMN milestones;
