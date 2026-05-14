-- name: CreateNodeType :one
INSERT INTO node_types (
    id, workspace_id, name, description, category, version,
    base_url, endpoint, method, content_type,
    headers_template, body_template,
    input_schema, output_schema,
    credential_id, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12,
    $13, $14,
    $15, now(), now()
) RETURNING *;

-- name: GetNodeType :one
SELECT * FROM node_types
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetNodeTypeByID :one
SELECT * FROM node_types
WHERE id = $1 AND active = true
LIMIT 1;

-- name: ListNodeTypes :many
SELECT * FROM node_types
WHERE workspace_id = $1
  AND active = true
  AND ($2::text = '' OR name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
  AND ($3::text = '' OR category = $3)
ORDER BY name ASC;

-- name: UpdateNodeType :one
UPDATE node_types SET
    name             = $3,
    description      = $4,
    category         = $5,
    base_url         = $6,
    endpoint         = $7,
    method           = $8,
    content_type     = $9,
    headers_template = $10,
    body_template    = $11,
    input_schema     = $12,
    output_schema    = $13,
    credential_id    = $14,
    updated_at       = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteNodeType :exec
DELETE FROM node_types
WHERE id = $1 AND workspace_id = $2;
