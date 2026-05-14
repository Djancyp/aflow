package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/djan/aflow/internal/executions/service"
	"github.com/djan/aflow/internal/nodes/interfaces"
	"github.com/djan/aflow/internal/nodes/registry"
	"github.com/djan/aflow/internal/observability/metrics"
	"github.com/djan/aflow/internal/runtime/engine"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("aflow/executor")

// CredentialDecryptor resolves $cred: references in node configs.
type CredentialDecryptor interface {
	Decrypt(ctx context.Context, workspaceID, credentialID string) (json.RawMessage, error)
}

// HTTPActionExecutor runs custom DB-backed HTTP-action node types.
type HTTPActionExecutor interface {
	Execute(ctx context.Context, workspaceID, nodeTypeID string, config map[string]any, input any) (any, error)
}

// Executor drives a full workflow execution: loads definition, runs DAG, persists state.
type Executor struct {
	repo           service.Repository
	registry       *registry.Registry
	credDecry      CredentialDecryptor // may be nil if creds not configured
	httpActionExec HTTPActionExecutor  // may be nil
}

func New(repo service.Repository, reg *registry.Registry, creds CredentialDecryptor) *Executor {
	return &Executor{repo: repo, registry: reg, credDecry: creds}
}

// WithHTTPActionExecutor attaches the HTTP-action executor for custom node types.
func (e *Executor) WithHTTPActionExecutor(h HTTPActionExecutor) *Executor {
	e.httpActionExec = h
	return e
}

// RunExecution loads the execution record, runs the DAG, and persists the final state.
func (e *Executor) RunExecution(ctx context.Context, executionID string) error {
	ctx, span := tracer.Start(ctx, "execution.run",
		trace.WithAttributes(attribute.String("execution.id", executionID)),
	)
	defer span.End()

	wallStart := time.Now()

	exec, err := e.repo.GetExecutionInternal(ctx, executionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("load execution %s: %w", executionID, err)
	}

	span.SetAttributes(
		attribute.String("workflow.id", exec.WorkflowID),
		attribute.String("workspace.id", exec.WorkspaceID),
	)

	defRaw, err := e.repo.GetVersionDefinition(ctx, exec.WorkflowVersionID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("load version %s: %w", exec.WorkflowVersionID, err)
	}

	def, err := engine.ParseDefinition(defRaw)
	if err != nil {
		_ = e.repo.FinishExecution(ctx, executionID, service.StatusFailed, nil, err.Error())
		e.recordExecution(service.StatusFailed, wallStart)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	order, err := engine.TopologicalSort(def)
	if err != nil {
		_ = e.repo.FinishExecution(ctx, executionID, service.StatusFailed, nil, err.Error())
		e.recordExecution(service.StatusFailed, wallStart)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if err := e.repo.StartExecution(ctx, executionID); err != nil {
		return fmt.Errorf("start execution: %w", err)
	}

	e.writeLog(ctx, executionID, nil, "info", fmt.Sprintf("starting execution: %d nodes", len(order)), nil)

	outputs := make(map[string]any, len(order))
	var initialInput any
	_ = json.Unmarshal(exec.Input, &initialInput)

	for _, node := range order {
		nodeInput := engine.ParentOutputs(node.ID, def.Edges, outputs)
		if len(nodeInput) == 0 {
			nodeInput = map[string]any{"$": initialInput}
		}

		nodeID := node.ID
		nodeStart := time.Now()
		output, nodeErr := e.runNodeWithRetry(ctx, exec, node, nodeInput)
		nodeDurSec := time.Since(nodeStart).Seconds()

		meta, _ := json.Marshal(map[string]any{"duration_ms": int64(nodeDurSec * 1000)})

		if nodeErr != nil {
			metrics.NodeExecutionsTotal.WithLabelValues(node.Type, "failure").Inc()
			metrics.NodeExecutionDuration.WithLabelValues(node.Type).Observe(nodeDurSec)

			e.writeLog(ctx, executionID, &nodeID, "error",
				fmt.Sprintf("node %s failed: %s", nodeID, nodeErr.Error()),
				json.RawMessage(meta),
			)
			errMsg := fmt.Sprintf("node %s: %s", nodeID, nodeErr.Error())
			_ = e.repo.FinishExecution(ctx, executionID, service.StatusFailed, nil, errMsg)
			e.recordExecution(service.StatusFailed, wallStart)
			span.SetStatus(codes.Error, errMsg)
			return fmt.Errorf("node %s failed: %w", nodeID, nodeErr)
		}

		metrics.NodeExecutionsTotal.WithLabelValues(node.Type, "success").Inc()
		metrics.NodeExecutionDuration.WithLabelValues(node.Type).Observe(nodeDurSec)

		outputs[nodeID] = output
		e.writeLog(ctx, executionID, &nodeID, "info",
			fmt.Sprintf("node %s completed", nodeID),
			json.RawMessage(meta),
		)
	}

	finalOutput, _ := json.Marshal(outputs)
	if err := e.repo.FinishExecution(ctx, executionID, service.StatusSuccess, finalOutput, ""); err != nil {
		slog.Error("failed to mark execution success", "execution_id", executionID, "err", err)
	}

	e.recordExecution(service.StatusSuccess, wallStart)
	span.SetStatus(codes.Ok, "")
	e.writeLog(ctx, executionID, nil, "info", "execution completed successfully", nil)
	return nil
}

// runNodeWithRetry runs a node up to RetryConfig.MaxAttempts times.
func (e *Executor) runNodeWithRetry(ctx context.Context, exec *service.Execution, nc engine.NodeConfig, input any) (any, error) {
	maxAttempts := 1
	delayMS := 0
	if nc.Retry != nil && nc.Retry.MaxAttempts > 1 {
		maxAttempts = nc.Retry.MaxAttempts
		delayMS = nc.Retry.DelayMS
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			e.writeLog(ctx, exec.ID, &nc.ID, "warn",
				fmt.Sprintf("node %s retry %d/%d after error: %s", nc.ID, attempt, maxAttempts-1, lastErr),
				nil,
			)
			if delayMS > 0 {
				select {
				case <-time.After(time.Duration(delayMS) * time.Millisecond):
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}
		out, err := e.runNode(ctx, exec, nc, input)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (e *Executor) runNode(ctx context.Context, exec *service.Execution, nc engine.NodeConfig, input any) (any, error) {
	ctx, span := tracer.Start(ctx, "node.execute",
		trace.WithAttributes(
			attribute.String("node.id", nc.ID),
			attribute.String("node.type", nc.Type),
		),
	)
	defer span.End()

	// Custom DB-backed HTTP-action node (type is a UUID).
	if isUUID(nc.Type) && e.httpActionExec != nil {
		out, err := e.httpActionExec.Execute(ctx, exec.WorkspaceID, nc.Type, nc.Config, input)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return out, err
	}

	// Resolve $cred: references in config before passing to built-in node.
	resolvedConfig, err := e.resolveCredentials(ctx, exec.WorkspaceID, nc.Config)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	n, err := e.registry.Get(nc.Type)
	if err != nil {
		slog.Warn("unknown node type, using no-op", "type", nc.Type)
		n, _ = e.registry.Get("no-op")
	}
	if n == nil {
		return input, nil
	}

	ec := interfaces.ExecutionContext{
		WorkspaceID: exec.WorkspaceID,
		ExecutionID: exec.ID,
		NodeID:      nc.ID,
		Config:      resolvedConfig,
	}

	out, err := n.Execute(ctx, ec, input)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return out, err
}

func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// resolveCredentials replaces all "$cred:<uuid>" and "$cred:<uuid>.<field>"
// references in config with their decrypted values.
func (e *Executor) resolveCredentials(ctx context.Context, workspaceID string, config map[string]any) (map[string]any, error) {
	if e.credDecry == nil {
		return config, nil
	}
	return resolveMap(ctx, workspaceID, config, e.credDecry)
}

func resolveMap(ctx context.Context, workspaceID string, m map[string]any, d CredentialDecryptor) (map[string]any, error) {
	out := make(map[string]any, len(m))
	for k, v := range m {
		resolved, err := resolveValue(ctx, workspaceID, v, d)
		if err != nil {
			return nil, err
		}
		out[k] = resolved
	}
	return out, nil
}

func resolveValue(ctx context.Context, workspaceID string, v any, d CredentialDecryptor) (any, error) {
	switch val := v.(type) {
	case string:
		if strings.HasPrefix(val, "$cred:") {
			return resolveCredRef(ctx, workspaceID, strings.TrimPrefix(val, "$cred:"), d)
		}
		return val, nil
	case map[string]any:
		return resolveMap(ctx, workspaceID, val, d)
	default:
		return v, nil
	}
}

func resolveCredRef(ctx context.Context, workspaceID, ref string, d CredentialDecryptor) (any, error) {
	parts := strings.SplitN(ref, ".", 2)
	credID := parts[0]

	raw, err := d.Decrypt(ctx, workspaceID, credID)
	if err != nil {
		return nil, fmt.Errorf("resolve credential %s: %w", credID, err)
	}

	if len(parts) == 1 {
		var out any
		_ = json.Unmarshal(raw, &out)
		return out, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("credential %s: data is not a JSON object", credID)
	}
	field := parts[1]
	val, ok := obj[field]
	if !ok {
		return nil, fmt.Errorf("credential %s has no field %q", credID, field)
	}
	return val, nil
}

func (e *Executor) recordExecution(status service.ExecutionStatus, start time.Time) {
	s := string(status)
	metrics.ExecutionsTotal.WithLabelValues(s).Inc()
	metrics.ExecutionDuration.WithLabelValues(s).Observe(time.Since(start).Seconds())
}

func (e *Executor) writeLog(ctx context.Context, executionID string, nodeID *string, level, msg string, meta json.RawMessage) {
	if err := e.repo.WriteLog(ctx, service.WriteLogInput{
		ExecutionID: executionID,
		NodeID:      nodeID,
		Level:       level,
		Message:     msg,
		Metadata:    meta,
	}); err != nil {
		slog.Error("failed to write execution log", "err", err)
	}
}
