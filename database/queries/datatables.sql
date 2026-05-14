-- name: CreateDataTable :one
INSERT INTO data_tables (id, workspace_id, name, schema, created_at)
VALUES ($1, $2, $3, $4, now())
RETURNING *;

-- name: GetDataTable :one
SELECT * FROM data_tables
WHERE id = $1 AND workspace_id = $2
LIMIT 1;

-- name: ListDataTables :many
SELECT * FROM data_tables
WHERE workspace_id = $1
ORDER BY created_at DESC;

-- name: DeleteDataTable :exec
DELETE FROM data_tables
WHERE id = $1 AND workspace_id = $2;

-- name: InsertDataTableRow :one
INSERT INTO data_table_rows (id, table_id, workspace_id, data, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
RETURNING *;

-- name: ListDataTableRows :many
SELECT * FROM data_table_rows
WHERE table_id = $1 AND workspace_id = $2
ORDER BY created_at DESC
LIMIT 1000;

-- name: DeleteDataTableRow :exec
DELETE FROM data_table_rows
WHERE id = $1 AND table_id = $2 AND workspace_id = $3;
