package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/api"
	"github.com/wgh-control/bkl-vacation/internal/app"
	"github.com/wgh-control/bkl-vacation/internal/config"
	"github.com/wgh-control/bkl-vacation/internal/identity"
	"github.com/wgh-control/bkl-vacation/internal/platform"
	"github.com/wgh-control/bkl-vacation/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := migrate(); err != nil {
			slog.Error("migrate", "err", err.Error())
			os.Exit(1)
		}
		slog.Info("migrations applied")
		return
	}
	if len(os.Args) > 1 {
		slog.Error("unknown command", "cmd", os.Args[1])
		os.Exit(2)
	}
	serve()
}

func migrate() error {
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	return app.Migrate(ctx, pool)
}

func serve() {
	cfg := config.Load()
	if cfg.DatabaseURL == "" || cfg.KeycloakIssuer == "" || cfg.KeycloakClientID == "" {
		slog.Error("DATABASE_URL, KEYCLOAK_ISSUER, and KEYCLOAK_CLIENT_ID are required")
		os.Exit(1)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()
	repo := store.NewPostgres(pool)
	mods := app.Modules(app.NewServices(cfg, repo))
	httpSrv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: platform.Handler(platform.Deps{
			Auth: &identity.Validator{
				Issuer:   cfg.KeycloakIssuer,
				ClientID: cfg.KeycloakClientID,
			},
			Ready:   repo.Ping,
			CORS:    cfg.CORSOrigin,
			OpenAPI: api.OpenAPI,
			Modules: mods,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}
	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http", "err", err.Error())
			os.Exit(1)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
}
