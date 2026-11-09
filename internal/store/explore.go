package store

import (
	"context"
	"database/sql"
	"errors"
)

// ExploreScope is explicit: empty account means the local union of saved accounts,
// never an implicit currently-active account. Empty source means both supported sources.
type ExploreScope struct {
	AccountID string `json:"accountId"`
	Source    string `json:"source"`
}
type ExploreRecord struct {
	Source, ID, Kind, Title, AuthorID, Author, CoverURL, AvatarURL string
	PublishedAt                                                    int64
}

func (s *Store) ExploreInput(ctx context.Context, q ExploreScope) ([]ExploreRecord, error) {
	return s.exploreInput(ctx, q, nil)
}

// The optional barrier is used only by same-package concurrency tests.
func (s *Store) exploreInput(ctx context.Context, q ExploreScope, barrier func()) ([]ExploreRecord, error) {
	if q.Source != "" && q.Source != "douyin" && q.Source != "bilibili" {
		return nil, errors.New("INVALID_SCOPE")
	}
	tx, err := s.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if q.AccountID != "" {
		var source string
		if err = tx.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, q.AccountID).Scan(&source); err != nil {
			return nil, errors.New("INVALID_SCOPE")
		}
		if q.Source != "" && source != q.Source {
			return nil, errors.New("SCOPE_MISMATCH")
		}
	}
	from := ` FROM relations r JOIN works w ON w.id=r.work_id LEFT JOIN authors a ON a.id=w.author_id WHERE r.kind IN ('like','favorite','watch_later') AND w.source IN ('douyin','bilibili')`
	args := []any{}
	if q.AccountID != "" {
		from += ` AND r.account_id=?`
		args = append(args, q.AccountID)
	}
	if q.Source != "" {
		from += ` AND w.source=?`
		args = append(args, q.Source)
	}
	// The first read pins a WAL snapshot; every field below is from that same version.
	var total int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT w.id)`+from, args...).Scan(&total); err != nil {
		return nil, err
	}
	if barrier != nil {
		barrier()
	}
	rows, err := tx.QueryContext(ctx, `SELECT w.source,w.remote_id,w.kind,w.title,COALESCE(a.remote_id,''),COALESCE(a.nickname,''),COALESCE(w.cover_url,''),COALESCE(a.avatar_url,''),w.published_at`+from+` GROUP BY w.id ORDER BY w.source,w.remote_id`, args...)
	if err != nil {
		return nil, err
	}
	result := make([]ExploreRecord, 0, total)
	for rows.Next() {
		var r ExploreRecord
		if err = rows.Scan(&r.Source, &r.ID, &r.Kind, &r.Title, &r.AuthorID, &r.Author, &r.CoverURL, &r.AvatarURL, &r.PublishedAt); err != nil {
			rows.Close()
			return nil, err
		}
		result = append(result, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(result) != total {
		return nil, errors.New("SNAPSHOT_INCONSISTENT")
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
