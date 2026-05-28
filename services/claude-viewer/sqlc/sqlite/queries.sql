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

-- name: ListAllPlansWithTags :many
SELECT
    p.id,
    p.file_name,
    p.title,
    p.created_at,
    p.modified_at,
    p.file_size,
    p.word_count,
    COALESCE(t.tag_ids, '') AS tag_ids,
    COALESCE(t.tag_names, '') AS tag_names
FROM plans p
LEFT JOIN (
    SELECT
        pt.plan_id,
        GROUP_CONCAT(DISTINCT t.id) AS tag_ids,
        GROUP_CONCAT(DISTINCT t.name) AS tag_names
    FROM plan_tags pt
    JOIN tags t ON pt.tag_id = t.id
    GROUP BY pt.plan_id
) t ON t.plan_id = p.id
ORDER BY p.modified_at DESC;

-- name: ListAllPlansWithPagination :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
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

-- name: UpsertConnector :exec
INSERT INTO connectors (name, display_name, enabled)
VALUES (?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    display_name = excluded.display_name,
    updated_at = CURRENT_TIMESTAMP;

-- name: DeleteConnector :exec
DELETE FROM connectors WHERE name = ?;

-- name: GetConnectorByRole :one
SELECT * FROM connectors WHERE role = ? LIMIT 1;

-- name: SetConnectorRole :exec
UPDATE connectors SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ?;

-- name: ClearConnectorRole :exec
UPDATE connectors SET role = NULL, updated_at = CURRENT_TIMESTAMP WHERE role = ?;

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

-- Tag queries

-- name: InsertTag :one
INSERT INTO tags (name, description, color, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = ? LIMIT 1;

-- name: GetTagByID :one
SELECT * FROM tags WHERE id = ? LIMIT 1;

-- name: ListAllTags :many
SELECT * FROM tags ORDER BY name ASC;

-- name: UpdateTag :exec
UPDATE tags
SET name = ?, description = ?, color = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = ?;

-- Plan-Tag association queries

-- name: AddTagToPlan :exec
INSERT INTO plan_tags (plan_id, tag_id, assigned_at)
VALUES (?, ?, ?);

-- name: RemoveTagFromPlan :exec
DELETE FROM plan_tags WHERE plan_id = ? AND tag_id = ?;

-- name: RemoveAllTagsFromPlan :exec
DELETE FROM plan_tags WHERE plan_id = ?;

-- name: RemoveAllPlansFromTag :exec
DELETE FROM plan_tags WHERE tag_id = ?;

-- name: GetPlanTags :many
SELECT t.* FROM tags t
JOIN plan_tags pt ON t.id = pt.tag_id
WHERE pt.plan_id = ?
ORDER BY t.name ASC;

-- name: GetPlansWithTag :many
SELECT p.id, p.file_name, p.title, p.created_at, p.modified_at, p.file_size, p.word_count
FROM plans p
JOIN plan_tags pt ON p.id = pt.plan_id
WHERE pt.tag_id = ?
ORDER BY p.modified_at DESC;

-- name: GetTagPlanCounts :many
SELECT t.name, COUNT(DISTINCT pt.plan_id) AS plan_count
FROM tags t
LEFT JOIN plan_tags pt ON t.id = pt.tag_id
GROUP BY t.id, t.name
ORDER BY t.name ASC;

-- name: GetUntaggedPlanCount :one
SELECT COUNT(*) AS count FROM plans
WHERE id NOT IN (SELECT DISTINCT plan_id FROM plan_tags);

-- name: ListUntaggedPlans :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE id NOT IN (SELECT DISTINCT plan_id FROM plan_tags)
ORDER BY modified_at DESC;
