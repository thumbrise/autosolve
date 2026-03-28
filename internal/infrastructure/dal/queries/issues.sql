-- name: UpsertIssue :exec
INSERT INTO issues (repository_id, github_id, number, title, body, state,
                    is_pull_request, pr_url, pr_html_url, pr_diff_url, pr_patch_url,
                    github_created_at, github_updated_at, synced_at)
VALUES (?, ?, ?, ?, ?, ?,
        ?, ?, ?, ?, ?,
        ?, ?, ?)
ON CONFLICT(github_id) DO UPDATE SET repository_id     = excluded.repository_id,
                                     title             = excluded.title,
                                     body              = excluded.body,
                                     state             = excluded.state,
                                     is_pull_request   = excluded.is_pull_request,
                                     pr_url            = excluded.pr_url,
                                     pr_html_url       = excluded.pr_html_url,
                                     pr_diff_url       = excluded.pr_diff_url,
                                     pr_patch_url      = excluded.pr_patch_url,
                                     github_created_at = excluded.github_created_at,
                                     github_updated_at = excluded.github_updated_at,
                                     updated_at        = datetime('now'),
                                     synced_at         = excluded.synced_at;

-- name: GetLastUpdateTime :one
SELECT github_id, github_updated_at
FROM issues
WHERE repository_id = ?
ORDER BY github_updated_at DESC
LIMIT 1;

-- name: GetIssueByRepoAndNumber :one
SELECT id, number, title, body, state
FROM issues
WHERE repository_id = ? AND number = ?;

-- name: GetIssueByID :one
SELECT id, number, title, body, state
FROM issues
WHERE id = ?;

-- name: ListIssues :many
SELECT id, repository_id, number, title, state
FROM issues
ORDER BY number DESC;