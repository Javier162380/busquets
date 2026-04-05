CREATE TABLE IF NOT EXISTS plans (
    id SERIAL PRIMARY KEY,
    file_name TEXT NOT NULL UNIQUE,
    file_path TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    modified_at TIMESTAMPTZ NOT NULL,
    indexed_at TIMESTAMPTZ NOT NULL,
    file_size BIGINT NOT NULL,
    word_count BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_plans_modified_at ON plans(modified_at DESC);
CREATE INDEX IF NOT EXISTS idx_plans_title ON plans(title);

CREATE TABLE IF NOT EXISTS settings (
    variable_name TEXT NOT NULL PRIMARY KEY,
    variable_type TEXT NOT NULL,
    string_value TEXT,
    number_value DOUBLE PRECISION,
    boolean_value BOOLEAN,
    datetime_value TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS plan_versions (
    id SERIAL PRIMARY KEY,
    plan_id BIGINT NOT NULL,
    version_number BIGINT NOT NULL,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    word_count BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(plan_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_plan_versions_plan_id ON plan_versions(plan_id);

CREATE TABLE IF NOT EXISTS connectors (
    name TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS connector_settings (
    id SERIAL PRIMARY KEY,
    connector_name TEXT NOT NULL,
    setting_key TEXT NOT NULL,
    setting_value TEXT NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(connector_name, setting_key),
    FOREIGN KEY (connector_name) REFERENCES connectors(name) ON DELETE CASCADE
);
