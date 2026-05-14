-- name: CreateAPIKey :one
INSERT INTO api_keys (id, workspace_id, name, key_hash, created_at)
VALUES ($1, $2, $3, $4, now())
RETURNING *;

-- name: GetAPIKeyByHash :one
SELECT * FROM api_keys
WHERE key_hash = $1
LIMIT 1;

-- name: ListAPIKeys :many
SELECT * FROM api_keys
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: DeleteAPIKey :exec
DELETE FROM api_keys
WHERE id = $1 AND workspace_id = $2;
