package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	execrepo "github.com/djan/aflow/internal/executions/repository"
	execsvc "github.com/djan/aflow/internal/executions/service"
	"github.com/djan/aflow/internal/nodes/builtin"
	"github.com/djan/aflow/internal/nodes/httpaction"
	"github.com/djan/aflow/internal/nodes/registry"
	_ "github.com/djan/aflow/internal/observability/metrics"
	"github.com/djan/aflow/internal/observability/tracing"
	"github.com/djan/aflow/internal/queue/workers"
	"github.com/djan/aflow/internal/runtime/executor"
	"github.com/djan/aflow/internal/runtime/scheduler"
	wfrepo "github.com/djan/aflow/internal/workflows/repository"
	"github.com/djan/aflow/pkg/config"
	"github.com/djan/aflow/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, "aflow-worker", "1.0.0")
	if err != nil {
		slog.Error("tracing setup failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

	pool, err := database.NewPool(ctx, cfg.Database.DSN)
	if err != nil {
		slog.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	maxWorkers := cfg.Queue.Workers
	if maxWorkers <= 0 {
		maxWorkers = 4
	}

	// Node registry with all built-ins.
	reg := registry.New()
	reg.Register(&builtin.ManualTriggerNode{})
	reg.Register(&builtin.WebhookTriggerNode{})
	reg.Register(&builtin.CronTriggerNode{})
	reg.Register(&builtin.HTTPRequestNode{})
	reg.Register(&builtin.NoOpNode{})
	reg.Register(&builtin.DelayNode{})
	reg.Register(&builtin.ConditionNode{})
	reg.Register(&builtin.TransformNode{})

	// HTTP-action executor for custom DB-backed node types.
	// Credential decryptor is nil for now (pass credsvc if encryption is configured).
	httpActionExec := httpaction.New(pool, nil)

	execRepo := execrepo.New(pool)
	wfRepo := wfrepo.New(pool)

	execSvcRef := &execSvcRef{}
	schedRef := &schedulerRef{}

	riverWorkers := river.NewWorkers()
	exec := executor.New(execRepo, reg, nil).WithHTTPActionExecutor(httpActionExec)
	river.AddWorker(riverWorkers, workers.NewWorkflowExecuteWorker(exec))
	river.AddWorker(riverWorkers, workers.NewCronTriggerWorker(execSvcRef, schedRef, wfRepo))

	var riverClient *river.Client[pgx.Tx]
	riverClient, err = river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: maxWorkers}},
		Workers: riverWorkers,
	})
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	execSvcRef.svc = execsvc.New(execRepo, riverClient)
	schedRef.scheduler = scheduler.New(riverClient)

	if err := riverClient.Start(ctx); err != nil {
		slog.Error("river start failed", "err", err)
		os.Exit(1)
	}
	slog.Info("aflow worker started", "max_workers", maxWorkers)

	metricsAddr := fmt.Sprintf(":%d", cfg.Worker.MetricsPort)
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	metricsSrv := &http.Server{Addr: metricsAddr, Handler: mux}
	go func() {
		slog.Info("worker metrics listening", "addr", metricsAddr)
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("worker metrics error", "err", err)
		}
	}()

	<-ctx.Done()
	slog.Info("worker shutting down")
	if err := riverClient.Stop(context.Background()); err != nil {
		slog.Error("worker stop error", "err", err)
	}
	_ = metricsSrv.Shutdown(context.Background())
	slog.Info("worker stopped")
}

type execSvcRef struct{ svc *execsvc.Service }

func (r *execSvcRef) ExecuteWithTrigger(ctx context.Context, wsID, wfID, trigger string, input json.RawMessage) (any, error) {
	return r.svc.ExecuteWithTrigger(ctx, wsID, wfID, trigger, input)
}

type schedulerRef struct{ scheduler *scheduler.Scheduler }

func (r *schedulerRef) ScheduleCron(ctx context.Context, wsID, wfID, schedule string) error {
	return r.scheduler.ScheduleCron(ctx, wsID, wfID, schedule)
}
