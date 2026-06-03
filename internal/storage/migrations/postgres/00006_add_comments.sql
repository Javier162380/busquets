-- +goose Up
CREATE TABLE IF NOT EXISTS plan_comments (
    id SERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plan_comments_plan_id ON plan_comments(plan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_plan_comments_plan_id;
DROP TABLE IF EXISTS plan_comments;
