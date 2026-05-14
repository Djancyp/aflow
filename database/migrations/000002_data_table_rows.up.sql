CREATE TABLE IF NOT EXISTS data_table_rows (
    id           UUID PRIMARY KEY,
    table_id     UUID NOT NULL REFERENCES data_tables(id) ON DELETE CASCADE,
    workspace_id TEXT NOT NULL,
    data         JSONB NOT NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT now(),
    updated_at   TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_data_table_rows_table     ON data_table_rows(table_id);
CREATE INDEX IF NOT EXISTS idx_data_table_rows_workspace ON data_table_rows(workspace_id);
