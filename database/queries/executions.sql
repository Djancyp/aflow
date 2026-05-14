-- name: CreateExecution :one
INSERT INTO executions (
    id, workspace_id, workflow_id, workflow_version_id,
    status, trigger_source, input, created_at
)
VALUES ($1, $2, $3, $4, 'queued', $5, $6, now())
RETURNING *;

-- name: GetExecution :one
SELECT * FROM executions
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: GetExecutionInternal :one
SELECT * FROM executions
WHERE id = $1
LIMIT 1;

-- name: ListExecutionsByWorkspace :many
SELECT * FROM executions
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListExecutionsByWorkflow :many
SELECT * FROM executions
WHERE workflow_id = $1 AND workspace_id = $2
ORDER BY created_at DESC;

-- name: StartExecution :exec
UPDATE executions
SET status = 'running', started_at = now()
WHERE id = $1;

-- name: FinishExecution :exec
UPDATE executions
SET status = $2, finished_at = now(), output = $3, error = $4
WHERE id = $1;

-- name: CancelExecution :one
UPDATE executions
SET status = 'cancelled'
WHERE id = $1 AND workspace_id = $2 AND status IN ('pending', 'queued')
RETURNING *;

-- name: CreateExecutionLog :exec
INSERT INTO execution_logs (execution_id, node_id, level, message, metadata, created_at)
VALUES ($1, $2, $3, $4, $5, now());

-- name: ListExecutionLogs :many
SELECT * FROM execution_logs
WHERE execution_id = $1
ORDER BY created_at ASC, id ASC;
