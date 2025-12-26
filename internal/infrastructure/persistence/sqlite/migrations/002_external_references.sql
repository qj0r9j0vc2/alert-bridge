-- SQLite Schema Migration: External References
-- Version: 2
-- Date: 2025-12-25
-- Description: Refactor slack_message_id and pagerduty_incident_id into generic external_references JSON field

-- Add new external_references column
ALTER TABLE alerts ADD COLUMN external_references TEXT DEFAULT '{}';

-- Migrate existing data to new format
UPDATE alerts
SET external_references = (
    SELECT json_object(
        CASE
            WHEN slack_message_id IS NOT NULL THEN 'slack'
        END,
        slack_message_id,
        CASE
            WHEN pagerduty_incident_id IS NOT NULL THEN 'pagerduty'
        END,
        pagerduty_incident_id
    )
)
WHERE slack_message_id IS NOT NULL OR pagerduty_incident_id IS NOT NULL;

-- Drop old indexes
DROP INDEX IF EXISTS idx_alerts_slack_message_id;
DROP INDEX IF EXISTS idx_alerts_pagerduty_incident_id;

-- Note: We're keeping the old columns for now to allow rollback if needed
-- After successful deployment and verification, run:
-- ALTER TABLE alerts DROP COLUMN slack_message_id;
-- ALTER TABLE alerts DROP COLUMN pagerduty_incident_id;

-- Insert version 2
INSERT OR IGNORE INTO schema_version (version, applied_at)
VALUES (2, datetime('now'));
