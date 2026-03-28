-- +goose Up

CREATE TABLE IF NOT EXISTS goqite
(
    id       TEXT    NOT NULL PRIMARY KEY DEFAULT ('m_' || hex(randomblob(16))),
    created  TEXT    NOT NULL             DEFAULT (strftime('%Y-%m-%dT%H:%M:%f', 'now')),
    updated  TEXT    NOT NULL             DEFAULT (strftime('%Y-%m-%dT%H:%M:%f', 'now')),
    queue    TEXT    NOT NULL,
    body     BLOB    NOT NULL,
    timeout  TEXT    NOT NULL             DEFAULT (strftime('%Y-%m-%dT%H:%M:%f', 'now')),
    received INTEGER NOT NULL             DEFAULT 0,
    priority INTEGER NOT NULL             DEFAULT 0
);
CREATE INDEX IF NOT EXISTS goqite_queue_idx ON goqite (queue);

-- +goose Down

DROP TABLE IF EXISTS goqite;
