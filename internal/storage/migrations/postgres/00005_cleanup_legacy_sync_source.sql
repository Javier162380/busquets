-- +goose Up
-- Transfer version history from legacy plans (sync_source='') to the
-- corresponding re-synced plans (sync_source != '') with the same file_name.
UPDATE plan_versions pv
SET plan_id = new_plan.id
FROM plans AS old_plan
JOIN plans AS new_plan ON new_plan.file_name = old_plan.file_name AND new_plan.sync_source != ''
WHERE pv.plan_id         = old_plan.id
  AND old_plan.sync_source = '';

-- Transfer tags, ignoring any that already exist on the new plan.
INSERT INTO plan_tags (plan_id, tag_id, assigned_at)
SELECT
    new_plan.id,
    pt.tag_id,
    pt.assigned_at
FROM plan_tags AS pt
JOIN plans AS old_plan ON old_plan.id = pt.plan_id AND old_plan.sync_source = ''
JOIN plans AS new_plan ON new_plan.file_name = old_plan.file_name AND new_plan.sync_source != ''
ON CONFLICT (plan_id, tag_id) DO NOTHING;

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
