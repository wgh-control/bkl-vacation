package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Apply runs module migrations that are not yet recorded in schema_migration.
// Versions already in the ledger are skipped, so renaming a file's path
// without changing its version does not reapply it.
func Apply(ctx context.Context, pool *pgxpool.Pool, tables []Table) error {
	if err := validateTables(tables); err != nil {
		return err
	}
	if pool == nil {
		return errors.New("postgres pool is nil")
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migration (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}
	for _, table := range tables {
		version := strings.TrimSpace(table.Version)
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migration WHERE version = $1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		// pgx prepares a single statement, which cannot hold a whole SQL file.
		// The simple query protocol runs every statement in this transaction.
		result := tx.Conn().PgConn().Exec(ctx, table.SQL)
		if _, err := result.ReadAll(); err != nil {
			_ = result.Close()
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if err := result.Close(); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migration (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func validateTables(tables []Table) error {
	seen := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		version := strings.TrimSpace(table.Version)
		if version == "" {
			return errors.New("migration version is empty")
		}
		if strings.TrimSpace(table.SQL) == "" {
			return fmt.Errorf("migration %s has no SQL", version)
		}
		if _, ok := seen[version]; ok {
			return fmt.Errorf("duplicate migration version %s", version)
		}
		seen[version] = struct{}{}
	}
	return nil
}
