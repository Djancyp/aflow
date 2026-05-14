-- name: CreateWorkflow :one
INSERT INTO workflows (id, workspace_id, name, description, active, latest_version, created_at, updated_at)
VALUES ($1, $2, $3, $4, false, 1, now(), now())
RETURNING *;

-- name: GetWorkflow :one
SELECT * FROM workflows
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: ListWorkflows :many
SELECT * FROM workflows
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: UpdateWorkflow :one
UPDATE workflows
SET name = $3, description = $4, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: SetWorkflowActive :one
UPDATE workflows
SET active = $3, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: BumpWorkflowVersion :one
UPDATE workflows
SET latest_version = latest_version + 1, updated_at = now()
WHERE id = $1 AND workspace_id = $2
RETURNING *;

-- name: DeleteWorkflow :exec
DELETE FROM workflows
WHERE id = $1 AND workspace_id = $2;

-- name: CreateWorkflowVersion :one
INSERT INTO workflow_versions (id, workflow_id, version, definition, published, created_at)
VALUES ($1, $2, $3, $4, false, now())
RETURNING *;

-- name: GetWorkflowVersion :one
SELECT * FROM workflow_versions
WHERE id = $1 AND workflow_id = $2
LIMIT 1;

-- name: GetWorkflowVersionByNumber :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND version = $2
LIMIT 1;

-- name: ListWorkflowVersions :many
SELECT * FROM workflow_versions
WHERE workflow_id = $1
ORDER BY version DESC;

-- name: PublishWorkflowVersion :one
UPDATE workflow_versions
SET published = true
WHERE id = $1 AND workflow_id = $2
RETURNING *;

-- name: GetLatestPublishedVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND published = true
ORDER BY version DESC
LIMIT 1;

-- name: GetWorkflowVersionByID :one
SELECT * FROM workflow_versions
WHERE id = $1
LIMIT 1;

-- name: GetWorkflowByWebhookSecret :one
SELECT * FROM workflows
WHERE id = $1 AND webhook_secret = $2 AND active = true
LIMIT 1;

-- name: SetWebhookSecret :one
UPDATE workflows
SET webhook_secret = $3
WHERE id = $1 AND workspace_id = $2
RETURNING *;
