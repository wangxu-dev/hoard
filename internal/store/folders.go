package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Folder struct {
	ID        string
	Title     string
	Count     int
	Kind      string
	Position  int
	Extra     json.RawMessage
	UpdatedAt int64
}
type FolderWork struct {
	RemoteID    string
	FavoritedAt int64
}

// SaveFolderMembership preserves other folders and accounts when adding one page.
// Works must already have been saved; a missing work rolls back the entire page.
func (s *Store) SaveFolderMembership(ctx context.Context, accountID string, folder Folder, items []FolderWork) error {
	return s.saveFolderMembership(ctx, accountID, folder, items, false)
}

// ReplaceFolderMembership atomically upserts the folder and replaces its membership
// for accountID while preserving other folders and accounts.
func (s *Store) ReplaceFolderMembership(ctx context.Context, accountID string, folder Folder, items []FolderWork) error {
	return s.saveFolderMembership(ctx, accountID, folder, items, true)
}

func (s *Store) saveFolderMembership(ctx context.Context, accountID string, folder Folder, items []FolderWork, replace bool) error {
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
	kind := folder.Kind
	if kind == "" {
		kind = "favorite"
	}
	extra := string(folder.Extra)
	if extra == "" {
		extra = "{}"
	}
	updatedAt := folder.UpdatedAt
	if updatedAt == 0 {
		updatedAt = time.Now().Unix()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO folders(account_id,remote_id,title,remote_count,kind,position,extra,updated_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(account_id,remote_id) DO UPDATE SET title=excluded.title,remote_count=excluded.remote_count,kind=excluded.kind,position=excluded.position,extra=excluded.extra,updated_at=excluded.updated_at`, accountID, folder.ID, folder.Title, folder.Count, kind, folder.Position, extra, updatedAt); err != nil {
		return fmt.Errorf("save folder: %w", err)
	}
	if replace {
		if _, err := tx.ExecContext(ctx, `DELETE FROM folder_works WHERE account_id=? AND folder_id=?`, accountID, folder.ID); err != nil {
			return fmt.Errorf("clear folder members: %w", err)
		}
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

// PruneFolders removes folders and folder memberships for accountID whose IDs
// are not present in keepIDs. When keepIDs is empty, all folders for accountID are removed.
func (s *Store) PruneFolders(ctx context.Context, accountID string, keepIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(keepIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM folder_works WHERE account_id=?`, accountID); err != nil {
			return fmt.Errorf("prune folder works: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM folders WHERE account_id=?`, accountID); err != nil {
			return fmt.Errorf("prune folders: %w", err)
		}
		return tx.Commit()
	}
	placeholders := strings.Repeat(",?", len(keepIDs))[1:]
	args := make([]any, 0, 1+len(keepIDs))
	args = append(args, accountID)
	for _, id := range keepIDs {
		args = append(args, id)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM folder_works WHERE account_id=? AND folder_id NOT IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("prune folder works: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM folders WHERE account_id=? AND remote_id NOT IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("prune folders: %w", err)
	}
	return tx.Commit()
}
func (s *Store) Folders(ctx context.Context, accountID string) ([]Folder, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT remote_id,title,remote_count,kind,position,extra,updated_at FROM folders WHERE account_id=? ORDER BY position ASC, CASE WHEN title IN ('默认收藏夹','默认收藏') THEN 0 ELSE 1 END, length(remote_id) DESC, remote_id DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Folder{}
	for rows.Next() {
		var item Folder
		var extra string
		if err := rows.Scan(&item.ID, &item.Title, &item.Count, &item.Kind, &item.Position, &extra, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if extra != "" && extra != "{}" {
			item.Extra = json.RawMessage(extra)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
