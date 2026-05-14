package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/djan/aflow/database/migrations"
	"github.com/djan/aflow/pkg/config"
	"github.com/djan/aflow/pkg/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func main() {
	direction := flag.String("direction", "up", "Migration direction: up or down")
	flag.Parse()

	if *direction != "up" && *direction != "down" {
		slog.Error("invalid direction", "direction", *direction)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.Database.DSN)
	if err != nil {
		slog.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 1. River schema migrations.
	if err := runRiverMigrations(ctx, pool, *direction); err != nil {
		slog.Error("river migrations failed", "err", err)
		os.Exit(1)
	}

	// 2. aflow SQL migrations.
	if err := runAflowMigrations(ctx, pool, *direction); err != nil {
		slog.Error("aflow migrations failed", "err", err)
		os.Exit(1)
	}

	slog.Info("migrations complete", "direction", *direction)
}

func runRiverMigrations(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}

	dir := rivermigrate.DirectionUp
	if direction == "down" {
		dir = rivermigrate.DirectionDown
	}

	res, err := migrator.Migrate(ctx, dir, nil)
	if err != nil {
		return fmt.Errorf("river migrate %s: %w", direction, err)
	}
	for _, v := range res.Versions {
		slog.Info("river migration applied", "version", v.Version, "direction", direction)
	}
	return nil
}

func runAflowMigrations(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	// Ensure tracking table exists.
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS _aflow_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Load applied migrations.
	rows, err := pool.Query(ctx, `SELECT name FROM _aflow_migrations ORDER BY name`)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}
	applied := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		applied[name] = true
	}
	rows.Close()

	// List embedded SQL files.
	suffix := ".up.sql"
	if direction == "down" {
		suffix = ".down.sql"
	}

	var files []string
	err = fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, suffix) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk migrations: %w", err)
	}
	sort.Strings(files)
	if direction == "down" {
		// Apply down migrations in reverse order.
		for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
			files[i], files[j] = files[j], files[i]
		}
	}

	for _, file := range files {
		// Track key is the basename without suffix.
		base := strings.TrimSuffix(file, suffix)
		if applied[base] && direction == "up" {
			slog.Info("migration already applied, skipping", "file", file)
			continue
		}

		sql, err := fs.ReadFile(migrations.FS, file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", file, err)
		}

		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute %s: %w", file, err)
		}

		if direction == "up" {
			if _, err := tx.Exec(ctx, `INSERT INTO _aflow_migrations (name) VALUES ($1) ON CONFLICT DO NOTHING`, base); err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("record migration %s: %w", file, err)
			}
		} else {
			if _, err := tx.Exec(ctx, `DELETE FROM _aflow_migrations WHERE name = $1`, base); err != nil {
				_ = tx.Rollback(ctx)
				return fmt.Errorf("remove migration record %s: %w", file, err)
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit %s: %w", file, err)
		}

		slog.Info("migration applied", "file", file, "direction", direction)
	}
	return nil
}
