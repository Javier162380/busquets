-- +goose Up
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    session_uuid TEXT NOT NULL UNIQUE,
    project_path TEXT NOT NULL,
    project_name TEXT NOT NULL,
    jsonl_file_path TEXT NOT NULL,
    plan_id BIGINT,
    status TEXT NOT NULL DEFAULT 'active',
    message_count BIGINT NOT NULL DEFAULT 0,
    first_message_at TIMESTAMPTZ,
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cwd TEXT,
    git_branch TEXT,
    slug TEXT,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_uuid ON sessions(session_uuid);
CREATE INDEX IF NOT EXISTS idx_sessions_plan_id ON sessions(plan_id);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project_path);
CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);

CREATE TABLE IF NOT EXISTS session_messages (
    id SERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL,
    message_uuid TEXT NOT NULL,
    parent_uuid TEXT,
    message_type TEXT NOT NULL,
    message_subtype TEXT,
    content TEXT,
    role TEXT,
    timestamp TIMESTAMPTZ NOT NULL,
    cwd TEXT,
    git_branch TEXT,
    is_meta BOOLEAN DEFAULT FALSE,
    is_sidechain BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON session_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON session_messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_type ON session_messages(message_type);
CREATE INDEX IF NOT EXISTS idx_messages_uuid ON session_messages(message_uuid);

CREATE TABLE IF NOT EXISTS session_file_changes (
    id SERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL,
    message_id BIGINT,
    file_path TEXT NOT NULL,
    change_type TEXT NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES session_messages(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_file_changes_session ON session_file_changes(session_id);
CREATE INDEX IF NOT EXISTS idx_file_changes_detected ON session_file_changes(detected_at DESC);

CREATE TABLE IF NOT EXISTS session_todos (
    id SERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL,
    message_id BIGINT,
    content TEXT NOT NULL,
    status TEXT NOT NULL,
    active_form TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES session_messages(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_todos_session ON session_todos(session_id);
CREATE INDEX IF NOT EXISTS idx_todos_status ON session_todos(status);

-- +goose Down
DROP TABLE IF EXISTS session_todos;
DROP TABLE IF EXISTS session_file_changes;
DROP TABLE IF EXISTS session_messages;
DROP TABLE IF EXISTS sessions;
