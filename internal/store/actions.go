package store

import (
	"context"
	"fmt"
	"time"
)

// ConfirmWorkAction updates only a relation; stored work metadata stays intact.
func (s *Store) ConfirmWorkAction(ctx context.Context, accountID, remoteID, kind string, on bool) error {
	if kind != "like" && kind != "favorite" {
		return fmt.Errorf("invalid action kind")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var workID string
	var published int64
	err = tx.QueryRowContext(ctx, `SELECT w.id,w.published_at FROM works w JOIN accounts a ON a.source=w.source WHERE a.id=? AND w.remote_id=?`, accountID, remoteID).Scan(&workID, &published)
	if err != nil {
		return fmt.Errorf("resolve action work: %w", err)
	}
	if on {
		_, err = tx.ExecContext(ctx, `INSERT INTO relations(account_id,work_id,kind,seq,published_at,first_seen) SELECT ?,?,?,COALESCE(MAX(seq),0)+1,?,? FROM relations WHERE account_id=? AND kind=? ON CONFLICT(account_id,kind,work_id) DO NOTHING`, accountID, workID, kind, published, time.Now().Unix(), accountID, kind)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM relations WHERE account_id=? AND work_id=? AND kind=?`, accountID, workID, kind)
	}
	if err != nil {
		return fmt.Errorf("confirm action: %w", err)
	}
	return tx.Commit()
}
