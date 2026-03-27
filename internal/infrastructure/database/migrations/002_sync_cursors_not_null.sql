-- +goose Up

-- sync_cursors: make nullable cursor fields NOT NULL with sensible defaults.
-- SQLite does not support ALTER COLUMN, so we recreate the table.

CREATE TABLE sync_cursors_new
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

INSERT INTO sync_cursors_new (
    id, created_at, updated_at, repository_id,
    resource_type, since_updated_at, next_page, e_tag
)
SELECT
    id,
    created_at,
    updated_at,
    repository_id,
    resource_type,
    COALESCE(since_updated_at, '0001-01-01 00:00:00') AS since_updated_at,
    COALESCE(next_page, 1) AS next_page,
    COALESCE(e_tag, '') AS e_tag
FROM sync_cursors;

DROP TABLE sync_cursors;
ALTER TABLE sync_cursors_new RENAME TO sync_cursors;

CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_resource ON sync_cursors (repository_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_sync_cursors_since_updated_at ON sync_cursors (since_updated_at);

-- +goose Down

CREATE TABLE sync_cursors_old
(
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at       DATETIME,
    updated_at       DATETIME,
    repository_id    INTEGER     NOT NULL REFERENCES repositories (id),
    resource_type    VARCHAR(50) NOT NULL,
    since_updated_at DATETIME,
    next_page        INTEGER DEFAULT 1,
    e_tag            VARCHAR(255)
);

INSERT INTO sync_cursors_old (
    id, created_at, updated_at, repository_id,
    resource_type, since_updated_at, next_page, e_tag
)
SELECT
    id,
    created_at,
    updated_at,
    repository_id,
    resource_type,
    since_updated_at,
    next_page,
    e_tag
FROM sync_cursors;

DROP TABLE sync_cursors;
ALTER TABLE sync_cursors_old RENAME TO sync_cursors;

CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_resource ON sync_cursors (repository_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_sync_cursors_since_updated_at ON sync_cursors (since_updated_at);
