-- name: GetSyncCursor :one
SELECT id, created_at, updated_at, repository_id, resource_type, since_updated_at, next_page, e_tag
FROM sync_cursors
WHERE repository_id = ? AND resource_type = ?;

-- name: UpsertSyncCursor :exec
INSERT INTO sync_cursors (repository_id, resource_type, since_updated_at, next_page, e_tag, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))
ON CONFLICT (repository_id, resource_type)
DO UPDATE SET since_updated_at = excluded.since_updated_at,
              next_page        = excluded.next_page,
              e_tag            = excluded.e_tag,
              updated_at       = datetime('now');
