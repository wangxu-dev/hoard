package store

import (
	"context"
	"fmt"
	"strings"
)

// ActionOwners overlays confirmed personal state on indexed list membership.
func (s *Store) ActionOwners(ctx context.Context, q LibraryQuery, ids []string) (map[string][]string, error) {
	owners, err := s.LibraryOwners(ctx, q, ids)
	if err != nil || len(ids) == 0 || q.Kind == "watch_later" {
		return owners, err
	}
	column := "liked"
	if q.Kind == "favorite" {
		column = "favorited"
	} else if q.Kind != "like" {
		return nil, fmt.Errorf("invalid action kind")
	}
	where, args := "s."+column+" IS NOT NULL", []any{}
	if q.AccountID != "" {
		where += " AND s.account_id=?"
		args = append(args, q.AccountID)
	}
	if q.Source != "" {
		where += " AND w.source=?"
		args = append(args, q.Source)
	}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.read.QueryContext(ctx, `SELECT w.remote_id,s.account_id,s.`+column+` FROM work_account_state s JOIN works w ON w.id=s.work_id WHERE `+where+` AND w.remote_id IN (`+strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, account string
		var on bool
		if err := rows.Scan(&id, &account, &on); err != nil {
			return nil, err
		}
		filtered := []string{}
		for _, owner := range owners[id] {
			if owner != account {
				filtered = append(filtered, owner)
			}
		}
		if on {
			filtered = append(filtered, account)
		}
		owners[id] = filtered
	}
	return owners, rows.Err()
}
