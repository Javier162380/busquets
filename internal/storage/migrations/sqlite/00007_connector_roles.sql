-- +goose Up
CREATE TABLE connector_roles (
    connector_name TEXT NOT NULL REFERENCES connectors(name) ON DELETE CASCADE,
    role           TEXT NOT NULL,
    active         INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (connector_name, role)
);

INSERT INTO connector_roles (connector_name, role, active)
SELECT name, role, 1 FROM connectors WHERE role IS NOT NULL;

ALTER TABLE connectors DROP COLUMN role;

-- +goose Down
ALTER TABLE connectors ADD COLUMN role TEXT NULL;
UPDATE connectors
SET role = (SELECT cr.role FROM connector_roles cr WHERE cr.connector_name = connectors.name AND cr.active = 1 LIMIT 1);
DROP TABLE connector_roles;
