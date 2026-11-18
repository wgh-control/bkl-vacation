// Package migrations applies every module registered with the shared database.
package migrations

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wgh-control/bkl-vacation/internal/app"
)

// Apply runs each module's tables that are not yet recorded in schema_migration.
func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	return app.Migrate(ctx, pool)
}
