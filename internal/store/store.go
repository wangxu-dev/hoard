package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
	// read serves the queries the grid and covers fire in bursts on connections of
	// their own, so they neither wait on each other nor on a sync writing through db.
	read *sql.DB
}

// readers is how many connections the read pool keeps.
const readers = 4

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(FULL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	read, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=query_only(1)")
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open database reader: %w", err)
	}
	read.SetMaxOpenConns(readers)
	return &Store{db: db, read: read}, nil
}

func (s *Store) Close() error {
	s.read.Close()
	return s.db.Close()
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
		return fmt.Errorf("write setting: %w", err)
	}
	return nil
}

func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, key).Scan(&value); err != nil {
		return "", fmt.Errorf("read setting: %w", err)
	}
	return value, nil
}

func (s *Store) UpsertAccount(ctx context.Context, account Account) (string, error) {
	if account.ID == "" || account.Source == "" || account.RemoteID == "" || account.SessionName == "" {
		return "", fmt.Errorf("account identity and session are required")
	}
	var id string
	now := time.Now().Unix()
	err := s.db.QueryRowContext(ctx, `INSERT INTO accounts(id,source,remote_id,nickname,session_name,status,position,created_at,last_used_at)
VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(source,remote_id) DO UPDATE SET
nickname=excluded.nickname, status=excluded.status, last_used_at=excluded.last_used_at RETURNING id`,
		account.ID, account.Source, account.RemoteID, account.Nickname, account.SessionName, account.Status, account.Position, now, now).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert account: %w", err)
	}
	return id, nil
}

func (s *Store) Account(ctx context.Context, id string) (Account, error) {
	var account Account
	err := s.db.QueryRowContext(ctx, `SELECT id,source,COALESCE(remote_id,''),nickname,COALESCE(avatar_url,''),session_name,status,position FROM accounts WHERE id=?`, id).
		Scan(&account.ID, &account.Source, &account.RemoteID, &account.Nickname, &account.AvatarURL, &account.SessionName, &account.Status, &account.Position)
	if err != nil {
		return Account{}, fmt.Errorf("read account: %w", err)
	}
	return account, nil
}

func (s *Store) SyncState(ctx context.Context, accountID, kind string) (SyncState, error) {
	var state SyncState
	err := s.db.QueryRowContext(ctx, `SELECT phase,status,head_cursor,COALESCE(backfill_cursor,''),head_seq,pages,items FROM sync_state WHERE account_id=? AND kind=?`, accountID, kind).
		Scan(&state.Phase, &state.Status, &state.HeadCursor, &state.BackfillCursor, &state.HeadSeq, &state.Pages, &state.Items)
	if err != nil {
		return SyncState{}, fmt.Errorf("read sync state: %w", err)
	}
	return state, nil
}
