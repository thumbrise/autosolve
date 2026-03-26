-- name: UpsertRepository :one
INSERT INTO repositories (owner, name, created_at, updated_at)
VALUES (?, ?, datetime('now'), datetime('now'))
ON CONFLICT(owner, name) DO UPDATE SET updated_at = datetime('now')
RETURNING id;

-- name: GetByOwnerName :one
SELECT id
FROM repositories
WHERE owner = ? AND name = ?;
