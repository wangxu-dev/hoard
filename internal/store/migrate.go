package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaV1 string

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	switch version {
	case 0:
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS works; DROP TABLE IF EXISTS accounts; DROP TABLE IF EXISTS settings;`); err != nil {
			return fmt.Errorf("remove prototype tables: %w", err)
		}
		if _, err := tx.ExecContext(ctx, schemaV1); err != nil {
			return fmt.Errorf("create schema v1: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	case 1:
	default:
		return fmt.Errorf("unsupported schema version: %d", version)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
