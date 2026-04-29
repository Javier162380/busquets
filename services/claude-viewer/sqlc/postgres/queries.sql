-- name: InsertPlan :exec
INSERT INTO plans (file_name, file_path, title, content, created_at, modified_at, indexed_at, file_size, word_count)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdatePlan :exec
UPDATE plans
SET title = $1, content = $2, modified_at = $3, indexed_at = $4, file_size = $5, word_count = $6
WHERE file_name = $7;

-- name: GetPlanByFileName :one
SELECT * FROM plans WHERE file_name = $1 LIMIT 1;

-- name: GetPlanByID :one
SELECT * FROM plans WHERE id = $1 LIMIT 1;

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
    t.tag_ids,
    t.tag_names
FROM plans p
         LEFT JOIN (
    SELECT
        pt.plan_id,
        array_agg(DISTINCT t.id ORDER BY t.id)::int4[] AS tag_ids,
        array_agg(DISTINCT t.name ORDER BY t.name)::text[] AS tag_names

    FROM plan_tags pt
             JOIN tags t ON pt.tag_id = t.id
    GROUP BY pt.plan_id
) t ON t.plan_id = p.id
ORDER BY p.id;

-- name: ListAllPlansWithPagination :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
ORDER BY modified_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchPlans :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE title LIKE $1 OR content LIKE $2
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
    t.tag_ids,
    t.tag_names
FROM plans p
         LEFT JOIN (
    SELECT
        pt.plan_id,
        array_agg(DISTINCT t.id ORDER BY t.id)::int4[] AS tag_ids,
        array_agg(DISTINCT t.name ORDER BY t.name)::text[] AS tag_names
    FROM plan_tags pt
             JOIN tags t ON pt.tag_id = t.id
    GROUP BY pt.plan_id
) t ON t.plan_id = p.id
WHERE p.title LIKE $1 OR p.content LIKE $2
ORDER BY p.modified_at DESC;

-- name: SearchPlansWithPagination :many
SELECT id, file_name, title, created_at, modified_at, file_size, word_count
FROM plans
WHERE title LIKE $1 OR content LIKE $2
ORDER BY modified_at DESC
LIMIT $3 OFFSET $4;

-- name: DeletePlan :exec
DELETE FROM plans WHERE file_name = $1;

-- name: CountPlans :one
SELECT COUNT(*) FROM plans;

-- name: GetSettingByName :one
SELECT * FROM settings WHERE variable_name = $1 LIMIT 1;

-- name: UpsertSetting :exec
INSERT INTO settings (variable_name, variable_type, string_value, number_value, boolean_value, datetime_value)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT(variable_name) DO UPDATE SET
    variable_type = excluded.variable_type,
    string_value = excluded.string_value,
    number_value = excluded.number_value,
    boolean_value = excluded.boolean_value,
    datetime_value = excluded.datetime_value;

-- name: DeleteSetting :exec
DELETE FROM settings WHERE variable_name = $1;

-- name: InsertPlanVersion :exec
INSERT INTO plan_versions (plan_id, version_number, file_path, content, word_count, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetPlanVersionHistory :many
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = $1
ORDER BY version_number DESC
LIMIT $2 OFFSET $3;

-- name: GetPlanVersionByNumber :one
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = $1 AND version_number = $2
LIMIT 1;

-- name: GetLatestVersionNumber :one
SELECT COALESCE(MAX(version_number), 0) FROM plan_versions WHERE plan_id = $1;

-- name: GetVersionCount :one
SELECT COUNT(*) FROM plan_versions WHERE plan_id = $1;

-- name: DeleteVersionsOlderThan :exec
DELETE FROM plan_versions
WHERE plan_id = $1 AND version_number <= $2;

-- name: SearchVersionsByContent :many
SELECT id, plan_id, version_number, file_path, content, word_count, created_at
FROM plan_versions
WHERE plan_id = $1 AND content LIKE $2
ORDER BY version_number DESC;

-- Connector queries

-- name: GetConnectorByName :one
SELECT * FROM connectors WHERE name = $1;

-- name: ListConnectors :many
SELECT * FROM connectors ORDER BY name;

-- name: GetEnabledConnector :one
SELECT * FROM connectors WHERE enabled = TRUE LIMIT 1;

-- name: UpsertConnector :exec
INSERT INTO connectors (name, display_name, enabled)
VALUES ($1, $2, $3)
ON CONFLICT(name) DO UPDATE SET
    display_name = excluded.display_name,
    updated_at = NOW();

-- name: SetConnectorEnabled :exec
UPDATE connectors SET enabled = TRUE, updated_at = NOW() WHERE name = $1;

-- name: DisableAllConnectors :exec
UPDATE connectors SET enabled = FALSE, updated_at = NOW();

-- name: DeleteConnector :exec
DELETE FROM connectors WHERE name = $1;

-- Connector settings queries

-- name: GetConnectorSetting :one
SELECT * FROM connector_settings WHERE connector_name = $1 AND setting_key = $2;

-- name: ListConnectorSettings :many
SELECT * FROM connector_settings WHERE connector_name = $1;

-- name: UpsertConnectorSetting :exec
INSERT INTO connector_settings (connector_name, setting_key, setting_value, is_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT(connector_name, setting_key) DO UPDATE SET
    setting_value = excluded.setting_value,
    is_secret = excluded.is_secret;

-- name: DeleteConnectorSetting :exec
DELETE FROM connector_settings WHERE connector_name = $1 AND setting_key = $2;

-- name: DeleteAllConnectorSettings :exec
DELETE FROM connector_settings WHERE connector_name = $1;

-- Tag queries

-- name: InsertTag :one
INSERT INTO tags (name, description, color, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetTagByName :one
SELECT * FROM tags WHERE name = $1 LIMIT 1;

-- name: GetTagByID :one
SELECT * FROM tags WHERE id = $1 LIMIT 1;

-- name: ListAllTags :many
SELECT * FROM tags ORDER BY name ASC;

-- name: UpdateTag :exec
UPDATE tags
SET name = $1, description = $2, color = $3, updated_at = NOW()
WHERE id = $4;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = $1;

-- Plan-Tag association queries

-- name: AddTagToPlan :exec
INSERT INTO plan_tags (plan_id, tag_id, assigned_at)
VALUES ($1, $2, $3);

-- name: RemoveTagFromPlan :exec
DELETE FROM plan_tags WHERE plan_id = $1 AND tag_id = $2;

-- name: RemoveAllTagsFromPlan :exec
DELETE FROM plan_tags WHERE plan_id = $1;

-- name: RemoveAllPlansFromTag :exec
DELETE FROM plan_tags WHERE tag_id = $1;

-- name: GetPlanTags :many
SELECT t.* FROM tags t
JOIN plan_tags pt ON t.id = pt.tag_id
WHERE pt.plan_id = $1
ORDER BY t.name ASC;

-- name: GetPlansWithTag :many
SELECT p.id, p.file_name, p.title, p.created_at, p.modified_at, p.file_size, p.word_count
FROM plans p
JOIN plan_tags pt ON p.id = pt.plan_id
WHERE pt.tag_id = $1
ORDER BY p.modified_at DESC;

-- Background job queries

-- name: InsertJob :one
INSERT INTO background_jobs (id, plan_id, name, description, agent_provider, agent_config, status, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetJobByID :one
SELECT * FROM background_jobs WHERE id = $1 LIMIT 1;

-- name: GetJobByPlanID :many
SELECT * FROM background_jobs WHERE plan_id = $1 ORDER BY created_at DESC;

-- name: ListJobs :many
SELECT * FROM background_jobs
WHERE ($1::BIGINT IS NULL OR plan_id = $2)
  AND ($3::TEXT IS NULL OR status = $4)
ORDER BY created_at DESC
LIMIT $5 OFFSET $6;

-- name: ListJobsWithPlans :many
SELECT
    bj.*,
    p.file_name AS plan_name,
    p.file_path AS plan_path
FROM background_jobs bj
JOIN plans p ON bj.plan_id = p.id
WHERE ($1::BIGINT IS NULL OR bj.plan_id = $2)
  AND ($3::TEXT IS NULL OR bj.status = $4)
ORDER BY bj.created_at DESC
LIMIT $5 OFFSET $6;

-- name: UpdateJob :exec
UPDATE background_jobs
SET name = COALESCE($1, name),
    description = COALESCE($2, description),
    agent_config = COALESCE($3, agent_config),
    status = COALESCE($4, status),
    updated_at = $5
WHERE id = $6;

-- name: UpdateJobStatus :exec
UPDATE background_jobs
SET status = $1,
    last_run_at = COALESCE($2, last_run_at),
    updated_at = $3
WHERE id = $4;

-- name: DeleteJob :exec
DELETE FROM background_jobs WHERE id = $1;

-- Job execution queries

-- name: InsertExecution :one
INSERT INTO job_executions (id, job_id, execution_number, status, triggered_by, started_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetExecutionByID :one
SELECT * FROM job_executions WHERE id = $1 LIMIT 1;

-- name: GetLatestExecution :one
SELECT * FROM job_executions
WHERE job_id = $1
ORDER BY execution_number DESC
LIMIT 1;

-- name: GetNextExecutionNumber :one
SELECT COALESCE(MAX(execution_number), 0) + 1 AS next_number
FROM job_executions
WHERE job_id = $1;

-- name: ListExecutions :many
SELECT * FROM job_executions
WHERE ($1::UUID IS NULL OR job_id = $2)
ORDER BY started_at DESC
LIMIT $3 OFFSET $4;

-- name: ListExecutionsWithContext :many
SELECT
    je.*,
    bj.name AS job_name,
    p.file_name AS plan_name
FROM job_executions je
JOIN background_jobs bj ON je.job_id = bj.id
JOIN plans p ON bj.plan_id = p.id
WHERE ($1::UUID IS NULL OR je.job_id = $2)
ORDER BY je.started_at DESC
LIMIT $3 OFFSET $4;

-- name: UpdateExecution :exec
UPDATE job_executions
SET status = $1,
    started_at = COALESCE($2, started_at),
    completed_at = COALESCE($3, completed_at),
    exit_code = COALESCE($4, exit_code),
    output_log = COALESCE($5, output_log),
    error_message = COALESCE($6, error_message)
WHERE id = $7;

-- name: DeleteExecutionsByJobID :exec
DELETE FROM job_executions WHERE job_id = $1;

-- Scheduled job queries

-- name: InsertScheduledJob :one
INSERT INTO scheduled_jobs (id, job_id, scheduled_at, created_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetScheduledJobByJobID :one
SELECT * FROM scheduled_jobs WHERE job_id = $1 LIMIT 1;

-- name: ListDueScheduledJobs :many
SELECT * FROM scheduled_jobs
WHERE scheduled_at <= $1
  AND cancelled = FALSE
ORDER BY scheduled_at ASC;

-- name: CancelScheduledJob :exec
UPDATE scheduled_jobs
SET cancelled = TRUE
WHERE job_id = $1;

-- name: DeleteScheduledJob :exec
DELETE FROM scheduled_jobs WHERE job_id = $1;
