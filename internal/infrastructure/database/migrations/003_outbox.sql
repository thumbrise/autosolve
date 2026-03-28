-- +goose Up

-- 1. Rename resource_type → topic in sync_cursors (SQLite has no ALTER COLUMN).
CREATE TABLE sync_cursors_v3
(
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at       DATETIME,
    updated_at       DATETIME,
    repository_id    INTEGER      NOT NULL REFERENCES repositories (id),
    topic            VARCHAR(100) NOT NULL,
    since_updated_at DATETIME     NOT NULL DEFAULT '0001-01-01 00:00:00',
    next_page        INTEGER      NOT NULL DEFAULT 1,
    e_tag            VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO sync_cursors_v3 (
    id, created_at, updated_at, repository_id,
    topic, since_updated_at, next_page, e_tag
)
SELECT
    id, created_at, updated_at, repository_id,
    resource_type, since_updated_at, next_page, e_tag
FROM sync_cursors;

DROP TABLE sync_cursors;
ALTER TABLE sync_cursors_v3 RENAME TO sync_cursors;

CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_cursor_repo_topic ON sync_cursors (repository_id, topic);

-- 2. Outbox events table.
CREATE TABLE IF NOT EXISTS outbox_events
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    DATETIME     NOT NULL DEFAULT (datetime('now')),
    topic         VARCHAR(100) NOT NULL,
    resource_id   INTEGER      NOT NULL,
    repository_id INTEGER      NOT NULL REFERENCES repositories (id),
    processed_at  DATETIME
);

CREATE INDEX IF NOT EXISTS idx_outbox_pending
ON outbox_events (topic, processed_at)
WHERE processed_at IS NULL;

-- +goose Down

DROP TABLE IF EXISTS outbox_events;

-- Restore sync_cursors with resource_type.
CREATE TABLE sync_cursors_rollback
(
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at       DATETIME,
    updated_at       DATETIME,
    repository_id    INTEGER      NOT NULL REFERENCES repositories (id),
    resource_type    VARCHAR(50)  NOT NULL,
    since_updated_at DATETIME     NOT NULL DEFAULT '0001-01-01 00:00:00',
    next_page        INTEGER      NOT NULL DEFAULT 1,
    e_tag            VARCHAR(255) NOT NULL DEFAULT ''
);

INSERT INTO sync_cursors_rollback (
    id, created_at, updated_at, repository_id,
    resource_type, since_updated_at, next_page, e_tag
)
SELECT
    id, created_at, updated_at, repository_id,
    topic, since_updated_at, next_page, e_tag
FROM sync_cursors;

DROP TABLE sync_cursors;
ALTER TABLE sync_cursors_rollback RENAME TO sync_cursors;

CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_resource ON sync_cursors (repository_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_sync_cursors_since_updated_at ON sync_cursors (since_updated_at);