-- name: InsertPlan :exec
INSERT INTO plans (file_name, file_path, title, content, created_at, modified_at, indexed_at, file_size, word_count)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdatePlan :exec
UPDATE plans
SET title = ?, content = ?, modified_at = ?, indexed_at = ?, file_size = ?, word_count = ?
WHERE file_name = ?;

-- name: GetPlanByFileName :one
SELECT * FROM plans WHERE file_name = ? LIMIT 1;

-- name: GetPlanByID :one
SELECT * FROM plans WHERE id = ? LIMIT 1;

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

-- name: SearchPlans :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE title LIKE ? OR content LIKE ?
ORDER BY modified_at DESC;

-- name: SearchPlansWithTags :many
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
WHERE p.title LIKE ? OR p.content LIKE ?
ORDER BY p.modified_at DESC;

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

-- Background job queries

-- name: InsertJob :one
INSERT INTO background_jobs (id, plan_id, name, description, agent_provider, agent_config, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetJobByID :one
SELECT * FROM background_jobs WHERE id = ? LIMIT 1;

-- name: GetJobByPlanID :many
SELECT * FROM background_jobs WHERE plan_id = ? ORDER BY created_at DESC;

-- name: ListJobs :many
SELECT * FROM background_jobs
WHERE (? IS NULL OR plan_id = ?)
  AND (? IS NULL OR status = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListJobsWithPlans :many
SELECT
    bj.*,
    p.file_name AS plan_name,
    p.file_path AS plan_path
FROM background_jobs bj
JOIN plans p ON bj.plan_id = p.id
WHERE (? IS NULL OR bj.plan_id = ?)
  AND (? IS NULL OR bj.status = ?)
ORDER BY bj.created_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateJob :exec
UPDATE background_jobs
SET name = COALESCE(?, name),
    description = COALESCE(?, description),
    agent_config = COALESCE(?, agent_config),
    status = COALESCE(?, status),
    updated_at = ?
WHERE id = ?;

-- name: UpdateJobStatus :exec
UPDATE background_jobs
SET status = ?,
    last_run_at = COALESCE(?, last_run_at),
    updated_at = ?
WHERE id = ?;

-- name: DeleteJob :exec
DELETE FROM background_jobs WHERE id = ?;

-- Job execution queries

-- name: InsertExecution :one
INSERT INTO job_executions (id, job_id, execution_number, status, triggered_by, started_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetExecutionByID :one
SELECT * FROM job_executions WHERE id = ? LIMIT 1;

-- name: GetLatestExecution :one
SELECT * FROM job_executions
WHERE job_id = ?
ORDER BY execution_number DESC
LIMIT 1;

-- name: GetNextExecutionNumber :one
SELECT COALESCE(MAX(execution_number), 0) + 1 AS next_number
FROM job_executions
WHERE job_id = ?;

-- name: ListExecutions :many
SELECT * FROM job_executions
WHERE (? IS NULL OR job_id = ?)
ORDER BY started_at DESC
LIMIT ? OFFSET ?;

-- name: ListExecutionsWithContext :many
SELECT
    je.*,
    bj.name AS job_name,
    p.file_name AS plan_name
FROM job_executions je
JOIN background_jobs bj ON je.job_id = bj.id
JOIN plans p ON bj.plan_id = p.id
WHERE (? IS NULL OR je.job_id = ?)
ORDER BY je.started_at DESC
LIMIT ? OFFSET ?;

-- name: UpdateExecution :exec
UPDATE job_executions
SET status = ?,
    started_at = COALESCE(?, started_at),
    completed_at = COALESCE(?, completed_at),
    exit_code = COALESCE(?, exit_code),
    output_log = COALESCE(?, output_log),
    error_message = COALESCE(?, error_message)
WHERE id = ?;

-- name: DeleteExecutionsByJobID :exec
DELETE FROM job_executions WHERE job_id = ?;

-- Scheduled job queries

-- name: InsertScheduledJob :one
INSERT INTO scheduled_jobs (id, job_id, scheduled_at, created_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetScheduledJobByJobID :one
SELECT * FROM scheduled_jobs WHERE job_id = ? LIMIT 1;

-- name: ListDueScheduledJobs :many
SELECT * FROM scheduled_jobs
WHERE scheduled_at <= ?
  AND cancelled = 0
ORDER BY scheduled_at ASC;

-- name: CancelScheduledJob :exec
UPDATE scheduled_jobs
SET cancelled = 1
WHERE job_id = ?;

-- name: DeleteScheduledJob :exec
DELETE FROM scheduled_jobs WHERE job_id = ?;
