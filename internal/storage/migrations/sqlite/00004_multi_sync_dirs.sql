-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

ALTER TABLE plans RENAME TO plans_old;

CREATE TABLE plans (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name   TEXT NOT NULL,
    sync_source TEXT NOT NULL DEFAULT '',
    file_path   TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    indexed_at  TIMESTAMP NOT NULL,
    file_size   INTEGER NOT NULL,
    word_count  INTEGER NOT NULL DEFAULT 0,
    UNIQUE(file_name, sync_source)
);

INSERT INTO plans (id, file_name, sync_source, file_path, title, content,
                   created_at, modified_at, indexed_at, file_size, word_count)
    SELECT id, file_name, '', file_path, title, content,
           created_at, modified_at, indexed_at, file_size, word_count
    FROM plans_old;

DROP TABLE plans_old;

CREATE INDEX IF NOT EXISTS idx_plans_modified_at ON plans(modified_at DESC);
CREATE INDEX IF NOT EXISTS idx_plans_title ON plans(title);

PRAGMA foreign_keys = ON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

ALTER TABLE plans RENAME TO plans_new;

CREATE TABLE plans (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_name   TEXT NOT NULL UNIQUE,
    file_path   TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    created_at  TIMESTAMP NOT NULL,
    modified_at TIMESTAMP NOT NULL,
    indexed_at  TIMESTAMP NOT NULL,
    file_size   INTEGER NOT NULL,
    word_count  INTEGER NOT NULL DEFAULT 0
);

INSERT INTO plans (id, file_name, file_path, title, content,
                   created_at, modified_at, indexed_at, file_size, word_count)
    SELECT id, file_name, file_path, title, content,
           created_at, modified_at, indexed_at, file_size, word_count
    FROM plans_new
    GROUP BY file_name;

DROP TABLE plans_new;

CREATE INDEX IF NOT EXISTS idx_plans_modified_at ON plans(modified_at DESC);
CREATE INDEX IF NOT EXISTS idx_plans_title ON plans(title);

PRAGMA foreign_keys = ON;
-- +goose StatementEnd
