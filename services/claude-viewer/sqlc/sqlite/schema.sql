CREATE TABLE IF NOT EXISTS plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name TEXT NOT NULL UNIQUE,
    file_path TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    indexed_at TIMESTAMP NOT NULL,
    file_size INTEGER NOT NULL,
    word_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_plans_modified_at ON plans(modified_at DESC);
CREATE INDEX IF NOT EXISTS idx_plans_title ON plans(title);

CREATE TABLE IF NOT EXISTS settings (
    variable_name TEXT NOT NULL PRIMARY KEY,
    variable_type TEXT NOT NULL,
    string_value TEXT,
    number_value REAL,
    boolean_value BOOLEAN,
    datetime_value DATETIME
);

CREATE TABLE IF NOT EXISTS plan_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plan_id INTEGER NOT NULL,
    version_number INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    content TEXT NOT NULL,
    word_count INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL,
    UNIQUE(plan_id, version_number)
);

CREATE INDEX IF NOT EXISTS idx_plan_versions_plan_id ON plan_versions(plan_id);

CREATE TABLE IF NOT EXISTS connectors (
    name TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS connector_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    connector_name TEXT NOT NULL,
    setting_key TEXT NOT NULL,
    setting_value TEXT NOT NULL,
    is_secret BOOLEAN NOT NULL DEFAULT 0,
    UNIQUE(connector_name, setting_key),
    FOREIGN KEY (connector_name) REFERENCES connectors(name) ON DELETE CASCADE
);
