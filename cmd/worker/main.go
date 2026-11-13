package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/notify"
	"github.com/wgh-control/bkl-vacation/internal/org"
	"github.com/wgh-control/bkl-vacation/internal/portal"
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
	svc := portal.New(repo, cfg.KeycloakHRGroup, cfg.Claims)
	var directory org.Directory
	if cfg.KeycloakAdminClientID != "" && cfg.KeycloakIssuer != "" {
		directory = &org.Keycloak{
			Issuer: cfg.KeycloakIssuer, ClientID: cfg.KeycloakAdminClientID,
			ClientSecret: cfg.KeycloakAdminClientSecret, HRGroup: cfg.KeycloakHRGroup, Claims: cfg.Claims,
		}
	}
	var mail notify.Sender
	if cfg.SMTPHost != "" && cfg.SMTPFrom != "" {
		mail = notify.SMTP{
			Host: cfg.SMTPHost, Port: cfg.SMTPPort, Mode: cfg.SMTPTLS,
			User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
		}
	}
	w := &worker.Worker{
		Svc: svc, Repo: repo, Directory: directory, Mail: mail,
		SyncEvery: cfg.SyncInterval, NotifyEvery: cfg.NotifyInterval,
	}
	slog.Info("worker started")
	w.Run(ctx)
}
