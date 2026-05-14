# aflow

AI-native headless workflow automation platform inspired by n8n.  
Built for backend automations, AI agents, MCP tools, and SaaS embedding.

---

## Overview

aflow executes **DAG-based workflows** asynchronously via a queue-backed runtime.  
It is entirely API-first — no visual editor, no frontend dependency.

**Key properties**

- Headless — operated via REST, MCP, webhooks, or CLI
- Queue-based — every execution goes through River Queue, never raw goroutines
- Immutable versions — published workflow definitions never mutate; executions reference a specific version
- Multi-tenant — all resources are scoped to a `workspace_id`
- Credential-safe — secrets encrypted at rest with AES-256-GCM; never returned via API
- Observable — OpenTelemetry tracing, Prometheus metrics, structured JSON logs
- AI-native — node catalog discoverable and manageable via MCP; workflows generated from natural language

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26+ |
| API | Echo v4 |
| Database | PostgreSQL (pgx v5 + sqlc) |
| Queue | River Queue |
| Migrations | Custom runner + River |
| Config | Viper |
| Logging | slog (JSON) |
| Tracing | OpenTelemetry (OTLP HTTP) |
| Metrics | Prometheus |
| Auth | JWT (HS256) + API keys |
| Encryption | AES-256-GCM |
| Template engine | `{{input.field}}` + `{{credential.field}}` |
| Live reload | Air |
| API docs | Swagger UI (swaggo) |

---

## Architecture

```
External Clients / AI Agents
          │
    REST  │  MCP  │  Webhooks
          ▼
     Echo API Server  (:8080)
     Swagger UI        (:8080/docs)
          │
    ┌─────┴────────────────────┬────────────┐
    │                          │            │
 Workflows               Executions   Credentials
 (CRUD + versions        (service +   (AES-GCM +
  + publish/deactivate)   queue)       data tables)
    │
    ▼
 River Queue  (PostgreSQL-backed)
    │
    ▼
 Worker Process  (:9091 metrics)
    │
    ▼
 DAG Executor  →  Node Registry
    │               │
    │         ┌─────┴──────────────────────────────────────┐
    │         │  BUILT-IN (compiled Go)                     │
    │         │  trigger.manual  trigger.webhook             │
    │         │  trigger.cron    http-request                │
    │         │  condition       transform                   │
    │         │  delay           no-op                       │
    │         │                                             │
    │         │  CUSTOM (DB-backed HTTP-action nodes)        │
    │         │  Any service reachable via HTTP              │
    │         │  Templates: {{input.*}} {{credential.*}}     │
    │         └─────────────────────────────────────────────┘
    ▼
 Execution Logs  (per-node, structured)
```

---

## Prerequisites

- Go 1.26+
- PostgreSQL 15+
- `swag` CLI — `go install github.com/swaggo/swag/cmd/swag@latest`
- `air` CLI — `go install github.com/air-verse/air@latest`
- `golangci-lint` — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- `sqlc` — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`

---

## Quick Start

```bash
# 1. Clone and enter
git clone <repo>
cd aflow

# 2. Install Go dependencies
make tidy

# 3. Configure (copy and edit)
cp aflow.example.yaml aflow.yaml

# 4. Run migrations (creates all tables + River schema)
make migrate-up

# 5. Start server with live reload
AFLOW_AUTH_DISABLED=true make dev-server

# 6. Start worker in a second terminal
AFLOW_AUTH_DISABLED=true make dev-worker

# 7. Open Swagger UI
open http://localhost:8080/docs/index.html
```

---

## Configuration

Configuration is loaded from `aflow.yaml`, then overridden by environment variables with the `AFLOW_` prefix.

| Key | Env | Default | Description |
|---|---|---|---|
| `server.port` | `AFLOW_SERVER_PORT` | `8080` | HTTP listen port |
| `server.host` | `AFLOW_SERVER_HOST` | `0.0.0.0` | HTTP listen address |
| `database.dsn` | `AFLOW_DATABASE_DSN` | — | PostgreSQL connection string |
| `queue.workers` | `AFLOW_QUEUE_WORKERS` | `4` | River worker concurrency |
| `worker.metrics_port` | `AFLOW_WORKER_METRICS_PORT` | `9091` | Worker health/metrics port |
| `crypto.encryption_key` | `APP_ENCRYPTION_KEY` | — | 64-char hex AES-256 key (**required**) |
| `auth.jwt_secret` | `AFLOW_JWT_SECRET` | — | HMAC secret for JWT validation |
| — | `AFLOW_AUTH_DISABLED` | `false` | Set `true` to skip all auth (dev only) |
| — | `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP collector (e.g. `http://jaeger:4318`) |
| — | `OTEL_SDK_DISABLED` | `false` | Set `true` to disable all tracing |

**Generate an encryption key:**
```bash
openssl rand -hex 32
```

**Example `aflow.yaml`:**
```yaml
database:
  dsn: postgres://postgres:postgres@localhost:5432/aflow?sslmode=disable

queue:
  workers: 4

crypto:
  encryption_key: ""  # set via APP_ENCRYPTION_KEY env var
```

---

## Make Commands

```bash
make build        # compile → bin/server, bin/worker, bin/migrate
make dev-server   # live-reload server with Air
make dev-worker   # live-reload worker with Air
make run-server   # run server without live reload
make run-worker   # run worker without live reload
make migrate-up   # apply all pending DB + River migrations
make migrate-down # roll back all migrations
make test         # go test -race ./...
make lint         # golangci-lint run ./...
make tidy         # go mod tidy
make sqlc         # regenerate sqlc from database/queries/
make docs         # regenerate Swagger spec from handler annotations
```

---

## API

Base URL: `http://localhost:8080`  
Interactive docs: `http://localhost:8080/docs/index.html`

All `/v1/*` routes require:
- `X-Workspace-ID: <workspace>` header
- `Authorization: Bearer <token>` header (JWT or `aflow_` API key), unless `AFLOW_AUTH_DISABLED=true`

### Workflows

```
POST   /v1/workflows                       Create workflow (with initial definition)
GET    /v1/workflows                       List all workflows
GET    /v1/workflows/:id                   Get workflow
PUT    /v1/workflows/:id                   Update name/description
DELETE /v1/workflows/:id                   Delete workflow
POST   /v1/workflows/:id/publish           Validate + publish + activate trigger
POST   /v1/workflows/:id/deactivate        Deactivate (stops cron, rejects webhooks)
GET    /v1/workflows/:id/versions          List immutable versions
POST   /v1/workflows/:id/execute           Trigger manual execution
GET    /v1/workflows/:id/executions        List executions for a workflow
```

### Executions

```
GET    /v1/executions                      List executions (paginated: ?limit=20&offset=0)
GET    /v1/executions/:id                  Get execution status + output
GET    /v1/executions/:id/logs             Get per-node execution logs
POST   /v1/executions/:id/cancel           Cancel (pending/queued only)
POST   /v1/executions/:id/retry            Retry (creates new execution from same input)
```

### Node Types

```
GET    /v1/node-types                      Unified catalog: built-in + custom (?q= ?category=)
POST   /v1/node-types                      Create custom HTTP-action node type
GET    /v1/node-types/:id                  Get node type with full config and schemas
PUT    /v1/node-types/:id                  Update custom node type
DELETE /v1/node-types/:id                  Delete custom node type
```

### Credentials

```
POST   /v1/credentials                     Create (data encrypted, never returned again)
GET    /v1/credentials                     List (metadata only — no encrypted data)
GET    /v1/credentials/:id                 Get (metadata only)
DELETE /v1/credentials/:id                 Delete
```

### Data Tables

```
POST   /v1/tables                          Create table with JSON schema
GET    /v1/tables                          List tables
GET    /v1/tables/:id                      Get table
DELETE /v1/tables/:id                      Delete table + all rows
POST   /v1/tables/:id/rows                 Insert row
GET    /v1/tables/:id/rows                 List rows (max 1000)
PATCH  /v1/tables/:id/rows/:row_id         Update row data
DELETE /v1/tables/:id/rows/:row_id         Delete row
```

### Webhooks (no auth — secret in URL)

```
POST   /webhooks/:workflow_id?secret=<whsec_...>   Trigger workflow via webhook
```

### MCP (Model Context Protocol)

```
POST   /mcp                                JSON-RPC 2.0 endpoint (workspace-scoped)
```

### System

```
GET    /health                             Health check
GET    /metrics                            Prometheus metrics
GET    /docs/*                             Swagger UI
```

---

## Workflow Definition

Workflows are DAGs defined as JSON. The definition is stored in `workflow_versions.definition` and never mutated after publishing.

```json
{
  "nodes": [
    {
      "id": "trigger",
      "type": "trigger.cron",
      "config": { "schedule": "0 9 * * 1-5" }
    },
    {
      "id": "fetch",
      "type": "http-request",
      "config": {
        "url": "https://api.example.com/data",
        "method": "GET",
        "headers": { "Authorization": "Bearer $cred:my-cred-id.token" }
      }
    },
    {
      "id": "check",
      "type": "condition",
      "config": { "field": "fetch.body.status", "operator": "eq", "value": "active" }
    },
    {
      "id": "notify",
      "type": "<custom-node-type-uuid>",
      "config": { "channel": "#alerts", "text": "Status is active" },
      "retry": { "max_attempts": 3, "delay_ms": 1000 }
    }
  ],
  "edges": [
    { "from": "trigger", "to": "fetch" },
    { "from": "fetch",   "to": "check" },
    { "from": "check",   "to": "notify" }
  ]
}
```

**Node `type` field:**
- Built-in nodes: use the string type name (`http-request`, `condition`, `trigger.cron`, etc.)
- Custom HTTP-action nodes: use the UUID returned when the node type was created

---

## Built-in Nodes

| Type | Category | Description | Key config |
|---|---|---|---|
| `trigger.manual` | trigger | API / MCP trigger. Pass-through. | — |
| `trigger.webhook` | trigger | Inbound HTTP. Secret auto-generated on publish. | `method` |
| `trigger.cron` | trigger | Scheduled. Self-chaining River job. | `schedule` (5-field cron) |
| `http-request` | core | Outbound HTTP call. Parses JSON response. | `url`, `method`, `headers`, `body` |
| `condition` | logic | Boolean gate. Emits `{matched, input}`. | `field`, `operator`, `value` |
| `transform` | logic | Field mapping via dot-path notation. | `fields: {out: "in.path"}` |
| `delay` | logic | Pause. Context-aware (cancellable). | `duration_ms` (max 30 000) |
| `no-op` | utility | Pass-through. Fallback for unknown types. | — |

### Credential references in node config

Reference encrypted credentials directly in node config:

```json
{ "headers": { "Authorization": "Bearer $cred:abc123.api_key" } }
```

The executor decrypts at runtime and substitutes the value. The raw secret is never stored in the definition.

### Node retry

```json
{ "retry": { "max_attempts": 3, "delay_ms": 500 } }
```

---

## Custom Node Types (HTTP-action)

Custom nodes are workspace-scoped, stored in the DB, and executed as HTTP calls at runtime. They expose the same JSON Schema interface as built-in nodes — AI sees one consistent catalog.

### Template syntax

| Placeholder | Resolved from |
|---|---|
| `{{input.field}}` | Parent node output |
| `{{input.nested.key}}` | Nested dot-path in parent output |
| `{{credential.field}}` | Decrypted credential JSON field |

Templates work in: URL, endpoint path, headers, and body. The body engine is JSON-aware — `"{{input.count}}"` where `count` is an integer becomes the integer `42`, not the string `"42"`.

### Create a custom node

```bash
POST /v1/node-types
X-Workspace-ID: ws1
{
  "name": "Send Slack Message",
  "category": "communication",
  "base_url": "https://slack.com/api",
  "endpoint": "/chat.postMessage",
  "method": "POST",
  "headers_template": {
    "Authorization": "Bearer {{credential.token}}"
  },
  "body_template": "{\"channel\":\"{{input.channel}}\",\"text\":\"{{input.text}}\"}",
  "credential_id": "<slack-token-credential-uuid>",
  "input_schema": {
    "type": "object",
    "required": ["channel", "text"],
    "properties": {
      "channel": { "type": "string", "description": "Slack channel ID or name" },
      "text":    { "type": "string", "description": "Message text" }
    }
  }
}
```

Response includes `"id": "<uuid>"`. Use that UUID as `type` in any workflow definition.

### Update / Delete

```bash
PUT    /v1/node-types/<uuid>   # replace all fields
DELETE /v1/node-types/<uuid>   # remove (breaks workflows referencing it)
```

---

## Triggers

### Manual
```bash
curl -X POST http://localhost:8080/v1/workflows/<id>/execute \
  -H "X-Workspace-ID: ws1" \
  -d '{"input": {"key": "value"}}'
```

### Webhook
Publish a workflow with a `trigger.webhook` node — the publish response contains `webhook_url`:

```bash
curl -X POST "http://localhost:8080/webhooks/<workflow_id>?secret=whsec_..." \
  -d '{"email": "user@example.com"}'
```

### Cron
Publish a workflow with a `trigger.cron` node — the scheduler inserts a River job automatically.  
The cron chain is self-sustaining: each fire schedules the next.  
Deactivating the workflow stops the chain.

---

## Execution States

```
queued → running → success
                 → failed
       → cancelled
```

---

## Authentication

**JWT** — sign tokens with `AFLOW_JWT_SECRET` (HS256). Required claim: `workspace_id`.

```json
{ "sub": "user-id", "workspace_id": "ws_prod", "exp": 9999999999 }
```

**API keys** — stored as SHA-256 hash. Format: `aflow_<64-hex-chars>`.

**Dev bypass** — set `AFLOW_AUTH_DISABLED=true`. The `X-Workspace-ID` header is then used directly.

---

## MCP Integration

aflow exposes an MCP server at `POST /mcp` using JSON-RPC 2.0 (protocol `2024-11-05`).

### Available MCP Tools

| Tool | Description |
|---|---|
| `list_workflows` | List active, published workflows |
| `execute_workflow` | Trigger a workflow execution |
| `get_execution` | Get execution status and output |
| `get_execution_logs` | Get per-node logs |
| `list_node_types` | Browse node catalog (built-in + custom) with search |
| `get_node_type` | Full JSON Schema for a specific node |
| `create_node_type` | Define a new HTTP-action node (AI can build integrations) |
| `update_node_type` | Update an existing custom node type |
| `delete_node_type` | Remove a custom node type |

### Example — AI builds a Slack integration

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "create_node_type",
    "arguments": {
      "name": "Send Slack Message",
      "category": "communication",
      "base_url": "https://slack.com/api",
      "endpoint": "/chat.postMessage",
      "method": "POST",
      "headers_template": { "Authorization": "Bearer {{credential.token}}" },
      "body_template": "{\"channel\":\"{{input.channel}}\",\"text\":\"{{input.text}}\"}",
      "input_schema": {
        "type": "object",
        "required": ["channel", "text"],
        "properties": {
          "channel": { "type": "string" },
          "text":    { "type": "string" }
        }
      }
    }
  }
}
```

### Example — AI discovers available nodes

```json
{
  "method": "tools/call",
  "params": {
    "name": "list_node_types",
    "arguments": { "category": "communication" }
  }
}
```

---

## Observability

| Signal | Endpoint / Config |
|---|---|
| Structured logs | stdout (JSON) |
| Prometheus metrics | `GET /metrics` (server) · `GET :9091/metrics` (worker) |
| OTel traces | set `OTEL_EXPORTER_OTLP_ENDPOINT` |
| Swagger UI | `GET /docs/index.html` |

**Key metrics:**

```
aflow_http_requests_total{method, path, status}
aflow_http_request_duration_seconds{method, path}
aflow_executions_total{status}
aflow_execution_duration_seconds{status}
aflow_node_executions_total{node_type, status}
aflow_node_execution_duration_seconds{node_type}
aflow_queue_jobs_enqueued_total{kind}
```

---

## Project Structure

```
cmd/
  server/      HTTP API server + MCP server
  worker/      River Queue worker (DAG executor + cron scheduler)
  migrate/     DB migration runner (aflow SQL + River schema)

internal/
  api/         Echo handlers, middleware, routes
  auth/        API key repository
  credentials/ AES-256-GCM credential service
  datatables/  Internal structured storage (rows with PATCH support)
  executions/  Execution lifecycle service
  mcp/         MCP server + 9 tools
  nodes/
    builtin/   8 built-in node implementations + JSON schemas
    httpaction/ HTTP-action executor for custom DB-backed nodes
    interfaces/ Node interface (Execute + Metadata with InputSchema)
    registry/  Thread-safe node registry with catalog support
  nodetypes/   Custom node type CRUD + unified catalog service
  observability/ OTel tracing + Prometheus metrics
  queue/       River job types + workers (workflow.execute, cron.trigger)
  runtime/
    engine/    DAG parser + topological sort + FindTriggerNode
    executor/  DAG executor (built-in + custom nodes, retry, cred injection)
    scheduler/ Cron trigger scheduler (River-backed)
    template/  {{input.*}} + {{credential.*}} template engine (JSON-aware)
  workflows/   Workflow CRUD + versioning + publish/deactivate + webhook

database/
  migrations/  SQL migration files (up/down) — 4 migrations
  queries/     sqlc SQL queries
  sqlc/        Generated Go DB layer

docs/          Generated Swagger spec (regenerate with: make docs)
pkg/
  config/      Viper configuration
  crypto/      AES-256-GCM encryption
  database/    pgxpool factory
```

---

## Database Schema

| Table | Purpose |
|---|---|
| `workflows` | Workflow metadata + active status + webhook secret |
| `workflow_versions` | Immutable DAG definitions (JSONB) |
| `executions` | Execution records with status lifecycle |
| `execution_logs` | Per-node structured logs |
| `credentials` | AES-256-GCM encrypted secrets |
| `data_tables` | Table definitions with JSON schema |
| `data_table_rows` | JSONB rows with CRUD |
| `node_types` | Custom HTTP-action node definitions + templates |
| `api_keys` | SHA-256 hashed API keys |
| `_aflow_migrations` | Applied migration tracking |
| `river_*` | River Queue internal schema |

---

## Roadmap

- [ ] SQLite support for single-node deployments
- [ ] OpenAI / Anthropic built-in nodes
- [ ] PostgreSQL query node
- [ ] MCP Tool Call node
- [ ] Running execution cancellation (SIGINT to River job)
- [ ] Cursor-based pagination on list endpoints
- [ ] Workflow templates (AI-generated starters)
- [ ] Visual editor (React frontend)
- [ ] Distributed worker fleet with leader election
- [ ] Credential update endpoint
- [ ] Node type versioning (immutable like workflow versions)

---

## License

MIT
