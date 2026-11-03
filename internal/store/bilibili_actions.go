package store

import (
	"context"
	"fmt"
	"time"
)

type FolderSelection struct {
	Folder
	Selected bool
}

// ConfirmBilibiliAction commits personal state, membership and list ownership together.
func (s *Store) ConfirmBilibiliAction(ctx context.Context, accountID, remoteID, kind string, on bool, folders []FolderSelection) error {
	if kind != "like" && kind != "favorite" && kind != "watch_later" {
		return fmt.Errorf("invalid action kind")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var workID string
	if err := tx.QueryRowContext(ctx, `SELECT w.id FROM works w JOIN accounts a ON a.source=w.source WHERE a.id=? AND a.source='bilibili' AND w.remote_id=?`, accountID, remoteID).Scan(&workID); err != nil {
		return fmt.Errorf("resolve action work: %w", err)
	}
	if kind == "favorite" && folders != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM folder_works WHERE account_id=? AND work_id=?`, accountID, workID); err != nil {
			return err
		}
		selected := false
		for _, f := range folders {
			if f.ID == "" || f.Count < 0 {
				return fmt.Errorf("invalid folder")
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO folders(account_id,remote_id,title,remote_count) VALUES(?,?,?,?) ON CONFLICT(account_id,remote_id) DO UPDATE SET title=excluded.title,remote_count=excluded.remote_count`, accountID, f.ID, f.Title, f.Count); err != nil {
				return err
			}
			if f.Selected {
				selected = true
				if _, err := tx.ExecContext(ctx, `INSERT INTO folder_works(account_id,folder_id,work_id,favorited_at) VALUES(?,?,?,?)`, accountID, f.ID, workID, time.Now().Unix()); err != nil {
					return err
				}
			}
		}
		if selected != on {
			return fmt.Errorf("inconsistent favorite state")
		}
	}
	if kind != "watch_later" {
		column := "liked"
		if kind == "favorite" {
			column = "favorited"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO work_account_state(account_id,work_id,`+column+`,`+column+`_at) VALUES(?,?,?,?) ON CONFLICT(account_id,work_id) DO UPDATE SET `+column+`=excluded.`+column+`,`+column+`_at=excluded.`+column+`_at`, accountID, workID, on, time.Now().Unix()); err != nil {
			return err
		}
	}
	if err := confirmWorkAction(ctx, tx, accountID, remoteID, kind, on); err != nil {
		return err
	}
	return tx.Commit()
}
