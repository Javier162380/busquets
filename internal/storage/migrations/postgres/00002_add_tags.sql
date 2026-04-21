-- +goose Up
-- Create tags table
CREATE TABLE IF NOT EXISTS tags (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);

-- Create plan_tags junction table
CREATE TABLE IF NOT EXISTS plan_tags (
    plan_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plan_id, tag_id),
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_plan_tags_tag_id ON plan_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_plan_tags_plan_id ON plan_tags(plan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_plan_tags_plan_id;
DROP INDEX IF EXISTS idx_plan_tags_tag_id;
DROP TABLE IF EXISTS plan_tags;
DROP INDEX IF EXISTS idx_tags_name;
DROP TABLE IF EXISTS tags;
