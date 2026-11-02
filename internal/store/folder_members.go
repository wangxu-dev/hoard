package store

import (
	"context"
	"fmt"
)

// FolderMembers returns previously indexed membership timestamps for incremental traversal.
func (s *Store) FolderMembers(ctx context.Context, accountID, folderID string) (map[string]int64, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT w.remote_id,f.favorited_at FROM folder_works f JOIN works w ON w.id=f.work_id WHERE f.account_id=? AND f.folder_id=?`, accountID, folderID)
	if err != nil {
		return nil, fmt.Errorf("read folder members: %w", err)
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}
