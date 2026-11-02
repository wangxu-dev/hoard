package store

import (
	"context"
	"fmt"
	"strings"
)

// LibraryOwners returns the accounts associated with the requested list entries.
func (s *Store) LibraryOwners(ctx context.Context, q LibraryQuery, ids []string) (map[string][]string, error) {
	owners := make(map[string][]string)
	if len(ids) == 0 {
		return owners, nil
	}
	if len(ids) > 500 {
		return nil, fmt.Errorf("too many library entries")
	}
	from := `relations r JOIN works w ON w.id=r.work_id`
	where := `r.kind=?`
	args := []any{q.Kind}
	switch q.Kind {
	case "like", "favorite", "watch_later":
	case "music":
		from = `music_relations r JOIN musics w ON w.id=r.music_id`
		where, args = "1=1", nil
	default:
		return nil, fmt.Errorf("invalid list kind")
	}
	if q.AccountID != "" {
		where += " AND r.account_id=?"
		args = append(args, q.AccountID)
	}
	if q.Source != "" {
		where += " AND w.source=?"
		args = append(args, q.Source)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.read.QueryContext(ctx, `SELECT DISTINCT w.remote_id,a.id FROM `+from+` JOIN accounts a ON a.id=r.account_id WHERE `+where+` AND w.remote_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`) ORDER BY a.position,a.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list owners: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, accountID string
		if err := rows.Scan(&id, &accountID); err != nil {
			return nil, fmt.Errorf("read owner: %w", err)
		}
		owners[id] = append(owners[id], accountID)
	}
	return owners, rows.Err()
}
