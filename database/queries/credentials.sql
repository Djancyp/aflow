-- name: CreateCredential :one
INSERT INTO credentials (id, workspace_id, name, type, encrypted_data, created_at)
VALUES ($1, $2, $3, $4, $5, now())
RETURNING *;

-- name: GetCredential :one
SELECT * FROM credentials
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: ListCredentials :many
SELECT * FROM credentials
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: DeleteCredential :exec
DELETE FROM credentials
WHERE id = $1 AND workspace_id = $2;
