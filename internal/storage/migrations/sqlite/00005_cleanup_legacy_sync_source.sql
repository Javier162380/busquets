-- +goose Up
-- Transfer version history from legacy plans (sync_source='') to the
-- corresponding re-synced plans (sync_source != '') with the same file_name.
UPDATE plan_versions
SET plan_id = (
    SELECT new_plan.id
    FROM plans AS new_plan
    JOIN plans AS old_plan ON old_plan.file_name = new_plan.file_name
    WHERE old_plan.id       = plan_versions.plan_id
      AND old_plan.sync_source = ''
      AND new_plan.sync_source != ''
    LIMIT 1
)
WHERE plan_id IN (
    SELECT old_plan.id
    FROM plans AS old_plan
    WHERE old_plan.sync_source = ''
      AND EXISTS (
          SELECT 1 FROM plans AS new_plan
          WHERE new_plan.file_name    = old_plan.file_name
            AND new_plan.sync_source != ''
      )
);

-- Transfer tags, ignoring any that already exist on the new plan.
INSERT OR IGNORE INTO plan_tags (plan_id, tag_id, assigned_at)
SELECT
    (
        SELECT new_plan.id
        FROM plans AS new_plan
        WHERE new_plan.file_name    = old_plan.file_name
          AND new_plan.sync_source != ''
        LIMIT 1
    ),
    pt.tag_id,
    pt.assigned_at
FROM plan_tags AS pt
JOIN plans AS old_plan ON old_plan.id = pt.plan_id AND old_plan.sync_source = ''
WHERE EXISTS (
    SELECT 1 FROM plans AS new_plan
    WHERE new_plan.file_name    = old_plan.file_name
      AND new_plan.sync_source != ''
);

-- Delete orphaned legacy plans (FK cascade removes their now-empty plan_tags rows).
DELETE FROM plans
WHERE sync_source = ''
  AND EXISTS (
      SELECT 1 FROM plans AS new_plan
      WHERE new_plan.file_name    = plans.file_name
        AND new_plan.sync_source != ''
  );

-- +goose Down
-- Not reversible: the original sync_source='' rows and their associations are gone.
