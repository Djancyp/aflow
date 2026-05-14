//	@title						aflow API
//	@version					1.0.0
//	@description				AI-native headless workflow automation platform. DAG execution, MCP-native, queue-based.
//	@host						localhost:8080
//	@BasePath					/
//	@schemes					http https
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT (HS256, claim: workspace_id) or aflow_ API key. Set AFLOW_AUTH_DISABLED=true to skip in dev.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/djan/aflow/docs"
	"github.com/djan/aflow/internal/api/handlers"
	apimw "github.com/djan/aflow/internal/api/middleware"
	"github.com/djan/aflow/internal/api/routes"
	"github.com/djan/aflow/internal/auth/apikeys"
	credrepo "github.com/djan/aflow/internal/credentials/repository"
	credsvc "github.com/djan/aflow/internal/credentials/service"
	tbrepo "github.com/djan/aflow/internal/datatables/repository"
	tbsvc "github.com/djan/aflow/internal/datatables/service"
	execrepo "github.com/djan/aflow/internal/executions/repository"
	execsvc "github.com/djan/aflow/internal/executions/service"
	mcpserver "github.com/djan/aflow/internal/mcp/server"
	"github.com/djan/aflow/internal/mcp/tools"
	ntrepo "github.com/djan/aflow/internal/nodetypes/repository"
	ntsvc "github.com/djan/aflow/internal/nodetypes/service"
	_ "github.com/djan/aflow/internal/observability/metrics"
	"github.com/djan/aflow/internal/observability/tracing"
	"github.com/djan/aflow/internal/nodes/registry"
	"github.com/djan/aflow/internal/runtime/scheduler"
	wfrepo "github.com/djan/aflow/internal/workflows/repository"
	wfsvc "github.com/djan/aflow/internal/workflows/service"
	"github.com/djan/aflow/pkg/config"
	"github.com/djan/aflow/pkg/crypto"
	"github.com/djan/aflow/pkg/database"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
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

	encryptor, err := crypto.NewEncryptor(cfg.Crypto.EncryptionKey)
	if err != nil {
		slog.Error("encryption key invalid", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := tracing.Setup(ctx, "aflow-server", "1.0.0")
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
	slog.Info("database connected")

	riverClient, err := river.NewClient[pgx.Tx](riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		slog.Error("river client failed", "err", err)
		os.Exit(1)
	}

	sched := scheduler.New(riverClient)

	// Node registry (needed for catalog in nodetypes service).
	reg := registry.New()

	// Services.
	wfService := wfsvc.New(wfrepo.New(pool)).WithScheduler(sched)
	execService := execsvc.New(execrepo.New(pool), riverClient)
	credService := credsvc.New(credrepo.New(pool), encryptor)
	tableService := tbsvc.New(tbrepo.New(pool))
	ntService := ntsvc.New(ntrepo.New(pool), reg)
	apiKeyRepo := apikeys.NewRepository(pool)

	authMW := apimw.Auth(apimw.AuthConfig{
		JWTSecret: cfg.Auth.JWTSecret,
		Lookup:    apiKeyRepo,
	})

	mcp := mcpserver.New("aflow", "1.0.0")
	tools.Register(mcp, wfService, execService, ntService)

	wh := handlers.NewWorkflowHandler(wfService)
	eh := handlers.NewExecutionHandler(execService)
	ch := handlers.NewCredentialHandler(credService)
	th := handlers.NewDataTableHandler(tableService)
	mh := handlers.NewMCPHandler(mcp)
	wbh := handlers.NewWebhookHandler(wfService, execService)
	nth := handlers.NewNodeTypeHandler(ntService)

	e := echo.New()
	e.HideBanner = true
	routes.Register(e, wh, eh, ch, th, mh, wbh, nth, authMW)

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      e,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("aflow server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("server stopped")
}
