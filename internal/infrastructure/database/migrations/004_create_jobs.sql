-- +goose Up

CREATE TABLE IF NOT EXISTS jobs
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    DATETIME    NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME    NOT NULL DEFAULT (datetime('now')),
    repository_id INTEGER     NOT NULL REFERENCES repositories (id),
    issue_id      INTEGER     NOT NULL REFERENCES issues (id),
    type          VARCHAR(50) NOT NULL,
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    prompt        TEXT        NOT NULL DEFAULT '',
    model         VARCHAR(100),
    result        TEXT,
    attempts      INTEGER     NOT NULL DEFAULT 0,
    last_error    TEXT
);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);
CREATE INDEX IF NOT EXISTS idx_jobs_repository_id ON jobs (repository_id);
CREATE INDEX IF NOT EXISTS idx_jobs_issue_id ON jobs (issue_id);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs (type);
CREATE INDEX IF NOT EXISTS idx_jobs_status_created ON jobs (status, created_at);

-- +goose Down

DROP TABLE IF EXISTS jobs;
