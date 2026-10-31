-- +goose Up
ALTER TABLE plan_versions ADD COLUMN milestones TEXT[]; -- NULL = none

-- +goose Down
ALTER TABLE plan_versions DROP COLUMN milestones;
