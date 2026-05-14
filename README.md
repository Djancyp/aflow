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

---

## Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26+ |
| API | Echo v4 |
| Database | PostgreSQL (pgx v5 + sqlc) |
| Queue | River Queue |
| Migrations | Custom runner (golang-migrate compatible SQL) + River |
| Config | Viper |
| Logging | slog (JSON) |
| Tracing | OpenTelemetry (OTLP HTTP) |
| Metrics | Prometheus |
| Auth | JWT (HS256) + API keys |
| Encryption | AES-256-GCM |
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
          │
    ┌─────┴──────────┬────────────┐
    │                │            │
 Workflows      Executions   Credentials
 (CRUD +        (service +   (AES-GCM +
  versions)      queue)       data tables)
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
    │         ┌─────┴────────────────┐
    │         │ trigger.manual       │
    │         │ trigger.webhook      │
    │         │ trigger.cron         │
    │         │ http-request         │
    │         │ condition            │
    │         │ transform            │
    │         │ delay                │
    │         │ no-op                │
    │         └──────────────────────┘
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

# 5. Start server (live reload)
AFLOW_AUTH_DISABLED=true make dev-server

# 6. Start worker in a second terminal
AFLOW_AUTH_DISABLED=true make dev-worker

# 7. Open Swagger UI
open http://localhost:8080/docs/index.html
```

---

## Configuration

Configuration is loaded from `aflow.yaml` in the working directory,  
then overridden by environment variables with the `AFLOW_` prefix.

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
| — | `OTEL_EXPORTER_OTLP_ENDPOINT` | — | OTLP collector endpoint (e.g. Jaeger) |
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
make migrate-up   # apply all pending migrations
make migrate-down # roll back all migrations
make test         # go test -race ./...
make lint         # golangci-lint run ./...
make tidy         # go mod tidy
make sqlc         # regenerate sqlc from database/queries/
make docs         # regenerate swagger spec from handler annotations
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
POST   /v1/workflows                    Create workflow (with initial definition)
GET    /v1/workflows                    List all workflows
GET    /v1/workflows/:id                Get workflow
PUT    /v1/workflows/:id                Update name/description
DELETE /v1/workflows/:id                Delete workflow
POST   /v1/workflows/:id/publish        Validate + publish + activate trigger
POST   /v1/workflows/:id/deactivate     Deactivate (stops cron chain, rejects webhooks)
GET    /v1/workflows/:id/versions       List immutable versions
POST   /v1/workflows/:id/execute        Trigger manual execution
GET    /v1/workflows/:id/executions     List executions for a workflow
```

### Executions

```
GET    /v1/executions                   List executions (paginated: ?limit=20&offset=0)
GET    /v1/executions/:id               Get execution status + output
GET    /v1/executions/:id/logs          Get per-node execution logs
POST   /v1/executions/:id/cancel        Cancel (pending/queued only)
POST   /v1/executions/:id/retry         Retry (creates new execution from same input)
```

### Credentials

```
POST   /v1/credentials                  Create (data encrypted, never returned again)
GET    /v1/credentials                  List (metadata only — no encrypted data)
GET    /v1/credentials/:id              Get (metadata only)
DELETE /v1/credentials/:id              Delete
```

### Data Tables

```
POST   /v1/tables                       Create table with JSON schema
GET    /v1/tables                       List tables
GET    /v1/tables/:id                   Get table
DELETE /v1/tables/:id                   Delete table + all rows
POST   /v1/tables/:id/rows              Insert row
GET    /v1/tables/:id/rows              List rows (max 1000)
DELETE /v1/tables/:id/rows/:row_id      Delete row
```

### Webhooks (no auth — secret in URL)

```
POST   /webhooks/:workflow_id?secret=<whsec_...>   Trigger workflow via webhook
```

### MCP (Model Context Protocol)

```
POST   /mcp                             JSON-RPC 2.0 endpoint (workspace-scoped)
```

MCP tools available: `list_workflows`, `execute_workflow`, `get_execution`, `get_execution_logs`

### System

```
GET    /health                          Health check
GET    /metrics                         Prometheus metrics
GET    /docs/*                          Swagger UI
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
      "id": "filter",
      "type": "condition",
      "config": { "field": "fetch.body.status", "operator": "eq", "value": "active" }
    },
    {
      "id": "notify",
      "type": "http-request",
      "config": {
        "url": "https://hooks.slack.com/services/...",
        "method": "POST"
      },
      "retry": { "max_attempts": 3, "delay_ms": 1000 }
    }
  ],
  "edges": [
    { "from": "trigger", "to": "fetch" },
    { "from": "fetch",   "to": "filter" },
    { "from": "filter",  "to": "notify" }
  ]
}
```

---

## Built-in Nodes

| Type | Description | Key config |
|---|---|---|
| `trigger.manual` | Manual API / MCP trigger. Pass-through. | — |
| `trigger.webhook` | Inbound HTTP trigger. Secret auto-generated on publish. | `method` |
| `trigger.cron` | Scheduled trigger via River Queue. Self-chaining. | `schedule` (5-field cron) |
| `http-request` | Outbound HTTP call. Parses JSON response. | `url`, `method`, `headers`, `body` |
| `condition` | Boolean gate. Emits `{matched, input}`. | `field`, `operator`, `value` |
| `transform` | Field mapping via dot-path notation. | `fields: {out: "in.path"}` |
| `delay` | Pause execution. Context-aware (cancellable). | `duration_ms` (max 30 000) |
| `no-op` | Pass-through. Fallback for unknown types. | — |

### Credential references in node config

Reference encrypted credentials directly in node config using `$cred:<id>.<field>`:

```json
{ "headers": { "Authorization": "Bearer $cred:abc123.api_key" } }
```

The executor decrypts the credential at runtime and substitutes the value. The raw secret is never stored in the workflow definition.

### Node retry

Any node can declare a retry policy:

```json
{ "retry": { "max_attempts": 3, "delay_ms": 500 } }
```

---

## Triggers

### Manual
```bash
curl -X POST http://localhost:8080/v1/workflows/<id>/execute \
  -H "X-Workspace-ID: ws1" \
  -H "Content-Type: application/json" \
  -d '{"input": {"key": "value"}}'
```

### Webhook
Publish a workflow with a `trigger.webhook` node — the publish response contains `webhook_url`:

```bash
curl -X POST "http://localhost:8080/webhooks/<workflow_id>?secret=whsec_..." \
  -H "Content-Type: application/json" \
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

- `queued` — job inserted into River Queue
- `running` — worker picked it up, DAG executing
- `success` — all nodes completed, outputs persisted
- `failed` — a node failed after all retries exhausted
- `cancelled` — cancelled while still queued (running executions cannot be cancelled yet)

---

## Authentication

**JWT** — sign tokens with `AFLOW_JWT_SECRET` (HS256). Required claim: `workspace_id`.

```json
{ "sub": "user-id", "workspace_id": "ws_prod", "exp": 9999999999 }
```

**API keys** — create via POST to `/v1/credentials` (separate table). Format: `aflow_<64-hex-chars>`.  
Stored as SHA-256 hash; the raw key is shown only at creation.

**Dev bypass** — set `AFLOW_AUTH_DISABLED=true` to skip all auth checks. The `X-Workspace-ID` header is then used directly (no signature validation).

---

## MCP Integration

aflow exposes an MCP server at `POST /mcp` using JSON-RPC 2.0 (protocol version `2024-11-05`).

```bash
curl -X POST http://localhost:8080/mcp \
  -H "X-Workspace-ID: ws1" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "execute_workflow",
      "arguments": { "workflow_id": "<uuid>", "input": {} }
    }
  }'
```

Available tools: `list_workflows`, `execute_workflow`, `get_execution`, `get_execution_logs`

---

## Observability

| Signal | Endpoint / Config |
|---|---|
| Structured logs | stdout (JSON) |
| Prometheus metrics | `GET /metrics` (server) · `GET :9091/metrics` (worker) |
| OTel traces | set `OTEL_EXPORTER_OTLP_ENDPOINT` (e.g. `http://jaeger:4318`) |
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
  server/      HTTP API server entry point
  worker/      River Queue worker entry point
  migrate/     Database migration runner

internal/
  api/         Echo handlers, middleware, routes
  auth/        API key repository
  credentials/ AES-256-GCM credential service
  datatables/  Internal structured storage
  executions/  Execution lifecycle service
  mcp/         MCP server + tools
  nodes/       Node interface, registry, built-ins
  observability/ Tracing + Prometheus metrics
  queue/       River job types + workers
  runtime/     DAG engine, executor, scheduler, trigger nodes
  workflows/   Workflow CRUD + versioning

database/
  migrations/  SQL migration files (up/down)
  queries/     sqlc SQL queries
  sqlc/        Generated Go DB layer

docs/          Generated Swagger spec (do not edit manually)
pkg/
  config/      Viper configuration
  crypto/      AES-256-GCM encryption
  database/    pgxpool factory
```

---

## Roadmap

- [ ] SQLite support for single-node deployments
- [ ] OpenAI / Anthropic built-in nodes
- [ ] PostgreSQL query node
- [ ] MCP Tool Call node
- [ ] Visual editor (React frontend)
- [ ] Workflow marketplace
- [ ] Distributed worker fleet with leader election
- [ ] Running execution cancellation
- [ ] Cursor-based pagination
- [ ] Workflow templates

---

## License

MIT
