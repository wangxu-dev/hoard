package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source,COALESCE(remote_id,''),nickname,COALESCE(avatar_url,''),session_name,status,position FROM accounts ORDER BY position,created_at,id`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()
	accounts := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Source, &a.RemoteID, &a.Nickname, &a.AvatarURL, &a.SessionName, &a.Status, &a.Position); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read accounts: %w", err)
	}
	return accounts, nil
}

func (s *Store) CreatePendingAccount(ctx context.Context, id string) (Account, error) {
	return s.CreatePendingSourceAccount(ctx, id, "douyin")
}

func (s *Store) CreatePendingSourceAccount(ctx context.Context, id, source string) (Account, error) {
	if source != "douyin" && source != "bilibili" {
		return Account{}, fmt.Errorf("unsupported account source")
	}
	now := time.Now().Unix()
	a := Account{ID: id, Source: source, SessionName: "hoard-account-" + id, Status: "pending"}
	_, err := s.db.ExecContext(ctx, `INSERT INTO accounts(id,source,session_name,status,position,created_at,last_used_at) VALUES(?,?,?,?,(SELECT COALESCE(MAX(position),-1)+1 FROM accounts),?,?)`, a.ID, a.Source, a.SessionName, a.Status, now, now)
	if err != nil {
		return Account{}, fmt.Errorf("create pending account: %w", err)
	}
	return a, nil
}

// ConfirmAccount also recognizes identities stored under their previous UID.
func (s *Store) ConfirmAccount(ctx context.Context, id, remoteID, previousUID, nickname, avatar string) (Account, error) {
	if remoteID == "" {
		return Account{}, fmt.Errorf("remote identity is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, fmt.Errorf("begin account confirmation: %w", err)
	}
	defer tx.Rollback()
	var source, pendingSession string
	if err := tx.QueryRowContext(ctx, `SELECT source,session_name FROM accounts WHERE id=?`, id).Scan(&source, &pendingSession); err != nil {
		return Account{}, fmt.Errorf("read pending account: %w", err)
	}
	target := id
	err = tx.QueryRowContext(ctx, `SELECT id FROM accounts WHERE source=? AND (remote_id=? OR (remote_id=? AND ?<>'')) ORDER BY (remote_id=?) DESC LIMIT 1`, source, remoteID, previousUID, previousUID, remoteID).Scan(&target)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Account{}, fmt.Errorf("find existing account: %w", err)
	}
	if target != id {
		var status, targetSession string
		if err := tx.QueryRowContext(ctx, `SELECT status,session_name FROM accounts WHERE id=?`, id).Scan(&status, &targetSession); err != nil {
			return Account{}, fmt.Errorf("read account status: %w", err)
		}
		if status != "pending" {
			return Account{}, fmt.Errorf("account identity changed")
		}
		if err := tx.QueryRowContext(ctx, `SELECT session_name FROM accounts WHERE id=?`, target).Scan(&targetSession); err != nil {
			return Account{}, fmt.Errorf("read existing session: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('session_cleanup:'||?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, target, targetSession); err != nil {
			return Account{}, fmt.Errorf("queue session cleanup: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id); err != nil {
			return Account{}, fmt.Errorf("remove pending account: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE accounts SET session_name=? WHERE id=?`, pendingSession, target); err != nil {
			return Account{}, fmt.Errorf("adopt login session: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE settings SET value=? WHERE key='active_account' AND value=?`, target, id); err != nil {
			return Account{}, fmt.Errorf("select existing account: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET remote_id=?,nickname=?,avatar_url=?,status='active',last_used_at=? WHERE id=?`, remoteID, nickname, avatar, time.Now().Unix(), target); err != nil {
		return Account{}, fmt.Errorf("confirm account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Account{}, fmt.Errorf("commit account confirmation: %w", err)
	}
	return s.Account(ctx, target)
}

func (s *Store) SetAccountStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET status=? WHERE id=?`, status, id)
	if err != nil {
		return fmt.Errorf("set account status: %w", err)
	}
	return nil
}

func (s *Store) ActivateAccount(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin activation: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE accounts SET last_used_at=? WHERE id=?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("touch account: %w", err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES('active_account',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, id); err != nil {
		return fmt.Errorf("save active account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account activation: %w", err)
	}
	return nil
}

func (s *Store) FinishSessionCleanup(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key=?`, "session_cleanup:"+id); err != nil {
		return fmt.Errorf("finish session cleanup: %w", err)
	}
	return nil
}

func (s *Store) SessionCleanups(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key,value FROM settings WHERE key LIKE 'session_cleanup:%'`)
	if err != nil {
		return nil, fmt.Errorf("read session cleanup: %w", err)
	}
	defer rows.Close()
	pending := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan session cleanup: %w", err)
		}
		pending[strings.TrimPrefix(key, "session_cleanup:")] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read session cleanup: %w", err)
	}
	return pending, nil
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin account deletion: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) SELECT 'session_cleanup:'||id,session_name FROM accounts WHERE id=? ON CONFLICT(key) DO UPDATE SET value=excluded.value`, id); err != nil {
		return fmt.Errorf("queue session cleanup: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key='active_account' AND value=?`, id); err != nil {
		return fmt.Errorf("clear active account: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit account deletion: %w", err)
	}
	return nil
}
