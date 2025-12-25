-- SQLite Schema for alert-bridge
-- Version: 1
-- Date: 2025-12-22

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY NOT NULL,
    applied_at TEXT NOT NULL
);

-- Insert version 1
INSERT OR IGNORE INTO schema_version (version, applied_at)
VALUES (1, datetime('now'));

-- ============================================================================
-- alerts table
-- ============================================================================
CREATE TABLE IF NOT EXISTS alerts (
    -- Primary Key
    id TEXT PRIMARY KEY NOT NULL,

    -- Identification
    fingerprint TEXT NOT NULL,

    -- Content
    name TEXT NOT NULL,
    instance TEXT NOT NULL DEFAULT '',
    target TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',

    -- Classification
    severity TEXT NOT NULL CHECK (severity IN ('critical', 'warning', 'info')),
    state TEXT NOT NULL CHECK (state IN ('active', 'acknowledged', 'resolved')),

    -- Structured Data (JSON)
    labels TEXT NOT NULL DEFAULT '{}',
    annotations TEXT NOT NULL DEFAULT '{}',

    -- External References
    slack_message_id TEXT DEFAULT NULL,
    pagerduty_incident_id TEXT DEFAULT NULL,

    -- Timestamps (RFC3339 format)
    fired_at TEXT NOT NULL,
    acked_at TEXT DEFAULT NULL,
    acked_by TEXT DEFAULT NULL,
    resolved_at TEXT DEFAULT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Indexes for alerts
CREATE INDEX IF NOT EXISTS idx_alerts_fingerprint
    ON alerts(fingerprint);

CREATE INDEX IF NOT EXISTS idx_alerts_slack_message_id
    ON alerts(slack_message_id)
    WHERE slack_message_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_alerts_pagerduty_incident_id
    ON alerts(pagerduty_incident_id)
    WHERE pagerduty_incident_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_alerts_state
    ON alerts(state);

CREATE INDEX IF NOT EXISTS idx_alerts_firing
    ON alerts(state)
    WHERE state IN ('active', 'acknowledged');

-- ============================================================================
-- ack_events table
-- ============================================================================
CREATE TABLE IF NOT EXISTS ack_events (
    -- Primary Key
    id TEXT PRIMARY KEY NOT NULL,

    -- Foreign Key
    alert_id TEXT NOT NULL,

    -- Source Information
    source TEXT NOT NULL CHECK (source IN ('slack', 'pagerduty', 'api')),

    -- User Information
    user_id TEXT NOT NULL DEFAULT '',
    user_email TEXT NOT NULL DEFAULT '',
    user_name TEXT NOT NULL DEFAULT '',

    -- Optional Data
    note TEXT DEFAULT NULL,
    duration_seconds INTEGER DEFAULT NULL,

    -- Timestamps
    created_at TEXT NOT NULL,

    -- Constraints
    FOREIGN KEY (alert_id) REFERENCES alerts(id) ON DELETE CASCADE
);

-- Indexes for ack_events
CREATE INDEX IF NOT EXISTS idx_ack_events_alert_id
    ON ack_events(alert_id);

CREATE INDEX IF NOT EXISTS idx_ack_events_alert_created
    ON ack_events(alert_id, created_at);

-- ============================================================================
-- silences table
-- ============================================================================
CREATE TABLE IF NOT EXISTS silences (
    -- Primary Key
    id TEXT PRIMARY KEY NOT NULL,

    -- Target Criteria (all optional - at least one should be set)
    alert_id TEXT DEFAULT NULL,
    instance TEXT DEFAULT NULL,
    fingerprint TEXT DEFAULT NULL,
    labels TEXT NOT NULL DEFAULT '{}',

    -- Duration
    start_at TEXT NOT NULL,
    end_at TEXT NOT NULL,

    -- Creator Information
    created_by TEXT NOT NULL DEFAULT '',
    created_by_email TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL CHECK (source IN ('slack', 'pagerduty', 'api')),

    -- Timestamps
    created_at TEXT NOT NULL
);

-- Indexes for silences
CREATE INDEX IF NOT EXISTS idx_silences_alert_id
    ON silences(alert_id)
    WHERE alert_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_silences_instance
    ON silences(instance)
    WHERE instance IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_silences_fingerprint
    ON silences(fingerprint)
    WHERE fingerprint IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_silences_end_at
    ON silences(end_at);

CREATE INDEX IF NOT EXISTS idx_silences_active
    ON silences(start_at, end_at);
