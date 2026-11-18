package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/app"
	"github.com/wgh-control/bkl-vacation/internal/config"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	if err := app.Migrate(ctx, pool); err != nil {
		slog.Error("migrate", "err", err.Error())
		os.Exit(1)
	}
	slog.Info("migrations applied")
}
