-- name: CreateJob :one
INSERT INTO jobs (repository_id, issue_id, type, status, prompt, model)
VALUES (?, ?, ?, 'pending', ?, ?)
RETURNING id, created_at;

-- name: GetJobByID :one
SELECT id, created_at, updated_at, repository_id, issue_id,
       type, status, prompt, model, result, attempts, last_error
FROM jobs
WHERE id = ?;

-- name: ListPendingJobs :many
SELECT id, created_at, updated_at, repository_id, issue_id,
       type, status, prompt, model, result, attempts, last_error
FROM jobs
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT ?;

-- name: MarkProcessing :exec
UPDATE jobs
SET status     = 'processing',
    updated_at = datetime('now')
WHERE id = ?
  AND status = 'pending';

-- name: MarkDone :exec
UPDATE jobs
SET status     = 'done',
    result     = ?,
    updated_at = datetime('now')
WHERE id = ?
  AND status = 'processing';

-- name: MarkFailed :exec
UPDATE jobs
SET status     = 'failed',
    last_error = ?,
    attempts   = attempts + 1,
    updated_at = datetime('now')
WHERE id = ?
  AND status = 'processing';

-- name: RetryFailedJobs :exec
UPDATE jobs
SET status     = 'pending',
    updated_at = datetime('now')
WHERE status = 'failed'
  AND attempts < ?;

-- name: ListJobsByIssue :many
SELECT id, created_at, updated_at, repository_id, issue_id,
       type, status, prompt, model, result, attempts, last_error
FROM jobs
WHERE issue_id = ?
ORDER BY created_at DESC;

-- name: ListDoneJobs :many
SELECT id, created_at, updated_at, repository_id, issue_id,
       type, status, prompt, model, result, attempts, last_error
FROM jobs
WHERE status = 'done'
ORDER BY updated_at ASC
LIMIT ?;
