-- name: InsertPlan :exec
INSERT INTO plans (file_name, file_path, title, content, created_at, modified_at, indexed_at, file_size, word_count)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdatePlan :exec
UPDATE plans
SET title = ?, content = ?, modified_at = ?, indexed_at = ?, file_size = ?, word_count = ?
WHERE file_name = ?;

-- name: GetPlanByFileName :one
SELECT * FROM plans WHERE file_name = ? LIMIT 1;

-- name: ListAllPlans :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
ORDER BY modified_at DESC;

-- name: ListAllPlansWithPagination :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
ORDER BY modified_at DESC
LIMIT ? OFFSET ?;

-- name: SearchPlans :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE title LIKE ? OR content LIKE ?
ORDER BY modified_at DESC;

-- name: SearchPlansWithPagination :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE title LIKE ? OR content LIKE ?
ORDER BY modified_at DESC
LIMIT ? OFFSET ?;

-- name: DeletePlan :exec
DELETE FROM plans WHERE file_name = ?;

-- name: CountPlans :one
SELECT COUNT(*) FROM plans;

-- name: GetSettingByName :one
SELECT * FROM settings WHERE variable_name = ? LIMIT 1;

-- name: UpsertSetting :exec
INSERT INTO settings (variable_name, variable_type, string_value, number_value, boolean_value, datetime_value)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(variable_name) DO UPDATE SET
    variable_type = excluded.variable_type,
    string_value = excluded.string_value,
    number_value = excluded.number_value,
    boolean_value = excluded.boolean_value,
    datetime_value = excluded.datetime_value;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE variable_name = ?;

-- name: InsertPlanVersion :exec
INSERT INTO plan_versions (plan_id, version_number, file_path, content, word_count, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetPlanVersionHistory :many
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = ?
ORDER BY version_number DESC
LIMIT ? OFFSET ?;

-- name: GetPlanVersionByNumber :one
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = ? AND version_number = ?
LIMIT 1;

-- name: GetLatestVersionNumber :one
SELECT COALESCE(MAX(version_number), 0) FROM plan_versions WHERE plan_id = ?;

-- name: GetVersionCount :one
SELECT COUNT(*) FROM plan_versions WHERE plan_id = ?;

-- name: DeleteVersionsOlderThan :exec
DELETE FROM plan_versions
WHERE plan_id = ? AND version_number <= ?;

-- name: SearchVersionsByContent :many
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = ? AND content LIKE ?
ORDER BY version_number DESC;

-- Connector queries

-- name: GetConnectorByName :one
SELECT * FROM connectors WHERE name = ?;

-- name: ListConnectors :many
SELECT * FROM connectors ORDER BY name;

-- name: GetEnabledConnector :one
SELECT * FROM connectors WHERE enabled = 1 LIMIT 1;

-- name: UpsertConnector :exec
INSERT INTO connectors (name, display_name, enabled)
VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    display_name = excluded.display_name,
    updated_at = CURRENT_TIMESTAMP;

-- name: SetConnectorEnabled :exec
UPDATE connectors SET enabled = 1, updated_at = CURRENT_TIMESTAMP WHERE name = ?;

-- name: DisableAllConnectors :exec
UPDATE connectors SET enabled = 0, updated_at = CURRENT_TIMESTAMP;

-- name: DeleteConnector :exec
DELETE FROM connectors WHERE name = ?;

-- Connector settings queries

-- name: GetConnectorSetting :one
SELECT * FROM connector_settings WHERE connector_name = ? AND setting_key = ?;

-- name: ListConnectorSettings :many
SELECT * FROM connector_settings WHERE connector_name = ?;

-- name: UpsertConnectorSetting :exec
INSERT INTO connector_settings (connector_name, setting_key, setting_value, is_secret)
VALUES (?, ?, ?, ?)
ON CONFLICT(connector_name, setting_key) DO UPDATE SET
    setting_value = excluded.setting_value,
    is_secret = excluded.is_secret;

-- name: DeleteConnectorSetting :exec
DELETE FROM connector_settings WHERE connector_name = ? AND setting_key = ?;

-- name: DeleteAllConnectorSettings :exec
DELETE FROM connector_settings WHERE connector_name = ?;
