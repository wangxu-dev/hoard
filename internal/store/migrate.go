package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaV1 string

//go:embed schema_v2.sql
var schemaV2 string

//go:embed schema_v3.sql
var schemaV3 string

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
	case 1, 2, 3:
	default:
		return fmt.Errorf("unsupported schema version: %d", version)
	}
	if version < 2 {
		if _, err := tx.ExecContext(ctx, schemaV2); err != nil {
			return fmt.Errorf("create folder schema: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}
	if version < 3 {
		if _, err := tx.ExecContext(ctx, schemaV3); err != nil {
			return fmt.Errorf("create personal state: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 3`); err != nil {
			return fmt.Errorf("set state schema version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
