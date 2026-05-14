CREATE TABLE IF NOT EXISTS node_types (
    id               UUID PRIMARY KEY,
    workspace_id     TEXT NOT NULL,
    name             TEXT NOT NULL,
    description      TEXT,
    category         TEXT NOT NULL DEFAULT 'custom',
    version          TEXT NOT NULL DEFAULT '1.0.0',

    -- HTTP-action target
    base_url         TEXT NOT NULL,
    endpoint         TEXT NOT NULL DEFAULT '/',
    method           TEXT NOT NULL DEFAULT 'POST',
    content_type     TEXT NOT NULL DEFAULT 'application/json',

    -- Template strings: {{input.field}} and {{credential.field}}
    headers_template JSONB,
    body_template    TEXT,

    -- JSON Schema draft-07
    input_schema     JSONB NOT NULL DEFAULT '{}',
    output_schema    JSONB,

    -- Credential bound at definition time
    credential_id    UUID REFERENCES credentials(id) ON DELETE SET NULL,

    active           BOOLEAN NOT NULL DEFAULT true,
    created_at       TIMESTAMP NOT NULL DEFAULT now(),
    updated_at       TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_node_types_workspace ON node_types(workspace_id);
CREATE INDEX IF NOT EXISTS idx_node_types_category  ON node_types(workspace_id, category);
