-- +goose Up
CREATE TABLE IF NOT EXISTS plan_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plan_comments_plan_id ON plan_comments(plan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_plan_comments_plan_id;
DROP TABLE IF EXISTS plan_comments;
