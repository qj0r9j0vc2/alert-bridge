-- MySQL Schema Migration: External References
-- Version: 2
-- Date: 2025-12-25
-- Description: Refactor slack_message_id and pagerduty_incident_id into generic external_references JSON field

-- Add new external_references column
ALTER TABLE alerts
ADD COLUMN external_references JSON DEFAULT '{}' AFTER annotations;

-- Migrate existing data to new format
UPDATE alerts
SET external_references = JSON_OBJECT(
    CASE WHEN slack_message_id IS NOT NULL THEN 'slack' ELSE NULL END,
    slack_message_id,
    CASE WHEN pagerduty_incident_id IS NOT NULL THEN 'pagerduty' ELSE NULL END,
    pagerduty_incident_id
)
WHERE slack_message_id IS NOT NULL OR pagerduty_incident_id IS NOT NULL;

-- Clean up NULL keys from JSON (MySQL JSON_OBJECT includes NULL keys)
UPDATE alerts
SET external_references = JSON_REMOVE(
    external_references,
    CASE WHEN slack_message_id IS NULL THEN '$.null' ELSE NULL END
)
WHERE external_references IS NOT NULL;

-- Drop old indexes
DROP INDEX idx_alerts_slack_message_id ON alerts;
DROP INDEX idx_alerts_pagerduty_incident_id ON alerts;

-- Note: We're keeping the old columns for now to allow rollback if needed
-- After successful deployment and verification, run:
-- ALTER TABLE alerts DROP COLUMN slack_message_id;
-- ALTER TABLE alerts DROP COLUMN pagerduty_incident_id;
