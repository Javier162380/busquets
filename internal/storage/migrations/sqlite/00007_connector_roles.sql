-- +goose Up
CREATE TABLE connector_roles (
    connector_name TEXT NOT NULL REFERENCES connectors(name) ON DELETE CASCADE,
    role           TEXT NOT NULL,
    PRIMARY KEY (connector_name, role),
    UNIQUE (role)
);

INSERT INTO connector_roles (connector_name, role)
SELECT name, role FROM connectors WHERE role IS NOT NULL;

ALTER TABLE connectors DROP COLUMN role;

-- +goose Down
ALTER TABLE connectors ADD COLUMN role TEXT NULL;
UPDATE connectors
SET role = (SELECT cr.role FROM connector_roles cr WHERE cr.connector_name = connectors.name LIMIT 1);
DROP TABLE connector_roles;
