-- +goose Up
CREATE TABLE IF NOT EXISTS plan_memories (
    id SERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL UNIQUE REFERENCES plans(id) ON DELETE CASCADE,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    summary TEXT NOT NULL,
    covers_up_to_version BIGINT NOT NULL DEFAULT -1,
    covers_up_to_comment_id BIGINT NOT NULL DEFAULT 0,
    generated_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_plan_memories_plan_id ON plan_memories(plan_id);

-- +goose Down
DROP INDEX IF EXISTS idx_plan_memories_plan_id;
DROP TABLE IF EXISTS plan_memories;
