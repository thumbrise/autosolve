-- +goose Up

CREATE TABLE IF NOT EXISTS repositories
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    owner      VARCHAR(255) NOT NULL,
    name       VARCHAR(255) NOT NULL,
    enabled    BOOLEAN DEFAULT 1,
    last_error TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_owner_name ON repositories (owner, name);
CREATE INDEX IF NOT EXISTS idx_repositories_enabled ON repositories (enabled);

CREATE TABLE IF NOT EXISTS users
(
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    github_id  INTEGER      NOT NULL,
    login      VARCHAR(255) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_github_id ON users (github_id);
CREATE INDEX IF NOT EXISTS idx_users_login ON users (login);

CREATE TABLE IF NOT EXISTS issues
(
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at        DATETIME          NOT NULL DEFAULT (datetime('now')),
    updated_at        DATETIME          NOT NULL DEFAULT (datetime('now')),
    repository_id     INTEGER           NOT NULL,
    github_id         INTEGER           NOT NULL,
    number            INTEGER           NOT NULL,
    title             TEXT              NOT NULL DEFAULT '',
    body              TEXT              NOT NULL DEFAULT '',
    state             VARCHAR(10)       NOT NULL,
    is_pull_request   BOOLEAN           NOT NULL DEFAULT 0,
    pr_url            TEXT,
    pr_html_url       TEXT,
    pr_diff_url       TEXT,
    pr_patch_url      TEXT,
    github_created_at DATETIME          NOT NULL,
    github_updated_at DATETIME          NOT NULL,
    synced_at         DATETIME          NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_issues_github_id ON issues (github_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_number ON issues (repository_id, number);
CREATE INDEX IF NOT EXISTS idx_issues_state ON issues (state);
CREATE INDEX IF NOT EXISTS idx_issues_is_pull_request ON issues (is_pull_request);
CREATE INDEX IF NOT EXISTS idx_issues_github_updated_at ON issues (github_updated_at);
CREATE INDEX IF NOT EXISTS idx_issues_synced_at ON issues (synced_at);

CREATE TABLE IF NOT EXISTS comments
(
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at        DATETIME,
    updated_at        DATETIME,
    issue_id          INTEGER  NOT NULL REFERENCES issues (id),
    github_id         INTEGER  NOT NULL,
    body              TEXT,
    author_login      VARCHAR(255),
    author_github_id  INTEGER,
    github_created_at DATETIME NOT NULL,
    github_updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_comments_github_id ON comments (github_id);
CREATE INDEX IF NOT EXISTS idx_comments_issue_id ON comments (issue_id);
CREATE INDEX IF NOT EXISTS idx_comments_author_github_id ON comments (author_github_id);
CREATE INDEX IF NOT EXISTS idx_comments_github_updated_at ON comments (github_updated_at);

CREATE TABLE IF NOT EXISTS labels
(
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at    DATETIME,
    updated_at    DATETIME,
    repository_id INTEGER      NOT NULL REFERENCES repositories (id),
    github_id     INTEGER      NOT NULL,
    name          VARCHAR(255) NOT NULL,
    color         VARCHAR(10),
    description   TEXT
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_labels_github_id ON labels (github_id);
CREATE INDEX IF NOT EXISTS idx_labels_repository_id ON labels (repository_id);

CREATE TABLE IF NOT EXISTS issue_labels
(
    issue_id INTEGER NOT NULL REFERENCES issues (id),
    label_id INTEGER NOT NULL REFERENCES labels (id),
    PRIMARY KEY (issue_id, label_id)
);

CREATE TABLE IF NOT EXISTS issue_assignees
(
    issue_id INTEGER NOT NULL REFERENCES issues (id),
    user_id  INTEGER NOT NULL REFERENCES users (id),
    PRIMARY KEY (issue_id, user_id)
);

CREATE TABLE IF NOT EXISTS sync_cursors
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_repo_resource ON sync_cursors (repository_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_sync_cursors_since_updated_at ON sync_cursors (since_updated_at);

-- +goose Down

DROP TABLE IF EXISTS sync_cursors;
DROP TABLE IF EXISTS issue_assignees;
DROP TABLE IF EXISTS issue_labels;
DROP TABLE IF EXISTS labels;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS issues;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS repositories;