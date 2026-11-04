package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SaveSyncPage commits content and its recovery position together.
func (s *Store) SaveSyncPage(ctx context.Context, accountID string, head bool, works []Work, musics []Music, record SyncRecord) error {
	if record.Kind != "like" && record.Kind != "favorite" && record.Kind != "music" {
		return fmt.Errorf("invalid sync kind")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync page: %w", err)
	}
	defer tx.Rollback()
	if record.Kind == "music" {
		_, err = saveMusics(ctx, tx, accountID, head, musics)
	} else {
		_, err = saveWorks(ctx, tx, accountID, record.Kind, head, works)
	}
	if err != nil {
		return err
	}
	if err = putSyncRecord(ctx, tx, accountID, record); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit sync page: %w", err)
	}
	return nil
}

type SyncCheckpoint struct {
	NextPage    int
	RemoteCount int
}

func (s *Store) SyncCheckpoint(ctx context.Context, accountID, scope string) (SyncCheckpoint, error) {
	var c SyncCheckpoint
	err := s.db.QueryRowContext(ctx, `SELECT next_page,remote_count FROM sync_checkpoints WHERE account_id=? AND scope=?`, accountID, scope).Scan(&c.NextPage, &c.RemoteCount)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	return c, err
}

// PutSyncCheckpoint follows durable page writes. Replaying an interrupted page is idempotent.
func (s *Store) PutSyncCheckpoint(ctx context.Context, accountID, scope string, c SyncCheckpoint) error {
	if c.NextPage == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM sync_checkpoints WHERE account_id=? AND scope=?`, accountID, scope)
		return err
	}
	if c.NextPage < 1 || c.RemoteCount < 0 {
		return fmt.Errorf("invalid sync checkpoint")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_checkpoints(account_id,scope,next_page,remote_count) VALUES(?,?,?,?) ON CONFLICT(account_id,scope) DO UPDATE SET next_page=excluded.next_page,remote_count=excluded.remote_count`, accountID, scope, c.NextPage, c.RemoteCount)
	return err
}
