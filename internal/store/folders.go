package store

import (
	"context"
	"fmt"
)

type Folder struct {
	ID    string
	Title string
	Count int
}
type FolderWork struct {
	RemoteID    string
	FavoritedAt int64
}

// SaveFolderMembership preserves other folders and accounts when adding one page.
// Works must already have been saved; a missing work rolls back the entire page.
func (s *Store) SaveFolderMembership(ctx context.Context, accountID string, folder Folder, items []FolderWork) error {
	if folder.ID == "" || folder.Count < 0 {
		return fmt.Errorf("invalid folder")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, accountID).Scan(&source); err != nil {
		return fmt.Errorf("folder account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO folders(account_id,remote_id,title,remote_count) VALUES(?,?,?,?) ON CONFLICT(account_id,remote_id) DO UPDATE SET title=excluded.title,remote_count=excluded.remote_count`, accountID, folder.ID, folder.Title, folder.Count); err != nil {
		return fmt.Errorf("save folder: %w", err)
	}
	for _, item := range items {
		if item.RemoteID == "" {
			return fmt.Errorf("missing work identity")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO folder_works(account_id,folder_id,work_id,favorited_at) VALUES(?,?,?,?) ON CONFLICT(account_id,folder_id,work_id) DO UPDATE SET favorited_at=excluded.favorited_at`, accountID, folder.ID, source+":"+item.RemoteID, item.FavoritedAt); err != nil {
			return fmt.Errorf("save folder member: %w", err)
		}
	}
	return tx.Commit()
}
func (s *Store) Folders(ctx context.Context, accountID string) ([]Folder, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT remote_id,title,remote_count FROM folders WHERE account_id=? ORDER BY remote_id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Folder{}
	for rows.Next() {
		var item Folder
		if err := rows.Scan(&item.ID, &item.Title, &item.Count); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
