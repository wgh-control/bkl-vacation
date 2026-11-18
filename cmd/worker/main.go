package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/app"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/org"
	"github.com/wgh-control/bkl-vacation/internal/store"
	"github.com/wgh-control/bkl-vacation/internal/worker"
)

func main() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	repo := store.NewPostgres(pool)
	mods := app.Modules(app.NewServices(cfg, repo))
	var directory org.Directory
	if cfg.KeycloakAdminClientID != "" && cfg.KeycloakIssuer != "" {
		directory = &org.Keycloak{
			Issuer: cfg.KeycloakIssuer, ClientID: cfg.KeycloakAdminClientID,
			ClientSecret: cfg.KeycloakAdminClientSecret, HRGroup: cfg.KeycloakHRGroup, Claims: cfg.Claims,
		}
	}
	w := &worker.Worker{
		Svc: app.Vacation(mods), Repo: repo, Directory: directory, Mail: app.Mail(cfg),
		Modules: mods, SyncEvery: cfg.SyncInterval, NotifyEvery: cfg.NotifyInterval,
	}
	slog.Info("worker started")
	w.Run(ctx)
}
