-- name: InsertOutboxEvent :exec
INSERT INTO outbox_events (topic, resource_id, repository_id)
VALUES (?, ?, ?);

-- name: PendingOutboxEvents :many
SELECT id, created_at, topic, resource_id, repository_id
FROM outbox_events
WHERE topic = ? AND repository_id = ? AND processed_at IS NULL
ORDER BY created_at ASC
LIMIT ?;

-- name: PendingOutboxEventsAll :many
SELECT id, created_at, topic, resource_id, repository_id
FROM outbox_events
WHERE topic = ? AND processed_at IS NULL
ORDER BY created_at ASC
LIMIT ?;

-- name: AckOutboxEvent :exec
UPDATE outbox_events
SET processed_at = datetime('now')
WHERE id = ? AND processed_at IS NULL;