-- MySQL Schema for Alert Bridge
-- Feature: feat/mysql-integration
-- Date: 2025-12-23
--
-- This schema supports multi-instance deployments with:
-- - Optimistic locking via version fields
-- - Native JSON columns for labels/annotations
-- - Foreign key constraints for referential integrity
-- - Comprehensive indexing for query performance

-- Character set and collation for full Unicode support (including emojis)
SET NAMES utf8mb4 COLLATE utf8mb4_unicode_ci;

-- ============================================================================
-- Schema Migrations Tracking
-- ============================================================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT PRIMARY KEY NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    INDEX idx_schema_migrations_applied_at (applied_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================================================
-- Alerts Table
-- ============================================================================

CREATE TABLE IF NOT EXISTS alerts (
    -- Primary Key
    id VARCHAR(255) PRIMARY KEY NOT NULL,

    -- Identification
    fingerprint VARCHAR(255) NOT NULL,

    -- Content
    name VARCHAR(255) NOT NULL,
    instance VARCHAR(255) NOT NULL DEFAULT '',
    target VARCHAR(255) NOT NULL DEFAULT '',
    summary TEXT NOT NULL,
    description TEXT NOT NULL,

    -- Classification (MySQL ENUM for storage efficiency)
    severity ENUM('critical', 'warning', 'info') NOT NULL,
    state ENUM('active', 'acknowledged', 'resolved') NOT NULL,

    -- Structured Data (MySQL JSON type for efficient querying)
    labels JSON NOT NULL,
    annotations JSON NOT NULL,

    -- External References
    slack_message_id VARCHAR(255) DEFAULT NULL,
    pagerduty_incident_id VARCHAR(255) DEFAULT NULL,

    -- Timestamps
    fired_at TIMESTAMP NOT NULL,
    acked_at TIMESTAMP NULL DEFAULT NULL,
    acked_by VARCHAR(255) DEFAULT NULL,
    resolved_at TIMESTAMP NULL DEFAULT NULL,

    -- Optimistic Locking (prevents lost updates in concurrent scenarios)
    version INT NOT NULL DEFAULT 1,

    -- Audit Timestamps (auto-maintained)
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    -- Indexes for query patterns from repository interface
    INDEX idx_alerts_fingerprint (fingerprint),
    INDEX idx_alerts_slack_message_id (slack_message_id),
    INDEX idx_alerts_pagerduty_incident_id (pagerduty_incident_id),
    INDEX idx_alerts_state (state),
    INDEX idx_alerts_created_at (created_at),
    INDEX idx_alerts_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Alert data with optimistic locking for multi-instance deployments';

-- ============================================================================
-- Acknowledgment Events Table
-- ============================================================================

CREATE TABLE IF NOT EXISTS ack_events (
    -- Primary Key
    id VARCHAR(255) PRIMARY KEY NOT NULL,

    -- Foreign Key (cascade delete - ack events are owned by alerts)
    alert_id VARCHAR(255) NOT NULL,

    -- Source Information (MySQL ENUM)
    source ENUM('slack', 'pagerduty', 'api') NOT NULL,

    -- User Information
    user_id VARCHAR(255) NOT NULL DEFAULT '',
    user_email VARCHAR(255) NOT NULL DEFAULT '',
    user_name VARCHAR(255) NOT NULL DEFAULT '',

    -- Optional Data
    note TEXT DEFAULT NULL,
    duration_seconds INT DEFAULT NULL,

    -- Audit Timestamp
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Foreign Key Constraint (cascade delete when alert is deleted)
    FOREIGN KEY (alert_id) REFERENCES alerts(id) ON DELETE CASCADE,

    -- Indexes for query patterns
    INDEX idx_ack_events_alert_id (alert_id),
    INDEX idx_ack_events_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Acknowledgment event audit trail';

-- ============================================================================
-- Silences Table
-- ============================================================================

CREATE TABLE IF NOT EXISTS silences (
    -- Primary Key
    id VARCHAR(255) PRIMARY KEY NOT NULL,

    -- Target Criteria (at least one should be set for matching)
    alert_id VARCHAR(255) DEFAULT NULL,
    instance VARCHAR(255) DEFAULT NULL,
    fingerprint VARCHAR(255) DEFAULT NULL,
    labels JSON NOT NULL,

    -- Duration (defines when silence is active)
    start_at TIMESTAMP NOT NULL,
    end_at TIMESTAMP NOT NULL,

    -- Creator Information
    created_by VARCHAR(255) NOT NULL DEFAULT '',
    created_by_email VARCHAR(255) NOT NULL DEFAULT '',
    reason TEXT NOT NULL,
    source ENUM('slack', 'pagerduty', 'api') NOT NULL,

    -- Optimistic Locking (for concurrent silence updates)
    version INT NOT NULL DEFAULT 1,

    -- Audit Timestamp
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Indexes for query patterns
    INDEX idx_silences_alert_id (alert_id),
    INDEX idx_silences_instance (instance),
    INDEX idx_silences_fingerprint (fingerprint),
    INDEX idx_silences_end_at (end_at),
    -- Composite index for time-range queries (FindActive)
    INDEX idx_silences_active (start_at, end_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
  COMMENT='Silence rules with time-based activation';

-- ============================================================================
-- Initial Migration Record
-- ============================================================================

INSERT INTO schema_migrations (version, applied_at)
VALUES (1, CURRENT_TIMESTAMP)
ON DUPLICATE KEY UPDATE applied_at = CURRENT_TIMESTAMP;

-- ============================================================================
-- Notes
-- ============================================================================

-- 1. OPTIMISTIC LOCKING:
--    UPDATE alerts SET ..., version = version + 1 WHERE id = ? AND version = ?
--    If RowsAffected == 0, concurrent update detected, retry required
--
-- 2. TRANSACTION ISOLATION:
--    Use READ COMMITTED for better concurrency (see research.md)
--    SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
--
-- 3. CONNECTION POOLING:
--    MaxOpenConns: 25 per instance
--    MaxIdleConns: 5 per instance
--    ConnMaxLifetime: 3 minutes
--    (See research.md for details)
--
-- 4. CHARACTER SET:
--    utf8mb4 required for emoji support in alert text
--    utf8mb4_unicode_ci for case-insensitive comparisons
--
-- 5. ENGINE:
--    InnoDB required for transactions, foreign keys, and row-level locking
--
-- 6. JSON FIELDS:
--    Use JSON_EXTRACT, JSON_CONTAINS for querying:
--    WHERE JSON_CONTAINS(labels, '"production"', '$.environment')
--
-- 7. INDEXES:
--    All indexes are non-unique (except PKs)
--    Composite index on (start_at, end_at) for time-range queries
--    Individual indexes for nullable foreign keys (slack_message_id, etc.)
