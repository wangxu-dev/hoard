package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) WriteWorkPage(ctx context.Context, page WorkPage) error {
	if page.Kind != "like" && page.Kind != "favorite" && page.Kind != "watch_later" {
		return fmt.Errorf("invalid work relation kind")
	}
	if page.Phase != "head" && page.Phase != "backfill" {
		return fmt.Errorf("invalid page phase")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin work page: %w", err)
	}
	defer tx.Rollback()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, page.AccountID).Scan(&source); err != nil {
		return fmt.Errorf("read page account: %w", err)
	}
	var upper, lower int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0),COALESCE(MIN(seq),0) FROM relations WHERE account_id=? AND kind=?`, page.AccountID, page.Kind).Scan(&upper, &lower); err != nil {
		return fmt.Errorf("read sequence bounds: %w", err)
	}
	// Reserve a descending range so each page keeps the source list order.
	seq := upper + int64(len(page.Works))
	if page.Phase == "backfill" {
		seq = lower - 1
	}
	now := time.Now().Unix()
	for _, work := range page.Works {
		if work.RemoteID == "" || (work.Kind != "video" && work.Kind != "gallery") {
			return fmt.Errorf("work identity and kind are required")
		}
		extra := work.Extra
		if len(extra) == 0 {
			extra = json.RawMessage(`{}`)
		}
		if !json.Valid(extra) {
			return fmt.Errorf("invalid work extra JSON")
		}
		var authorID any
		if work.Author != nil {
			if work.Author.RemoteID == "" {
				return fmt.Errorf("author identity is required")
			}
			authorID = source + ":" + work.Author.RemoteID
			if _, err := tx.ExecContext(ctx, `INSERT INTO authors(id,source,remote_id,nickname,avatar_url,updated_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET nickname=excluded.nickname,avatar_url=excluded.avatar_url,updated_at=excluded.updated_at`,
				authorID, source, work.Author.RemoteID, work.Author.Nickname, work.Author.AvatarURL, now); err != nil {
				return fmt.Errorf("upsert author: %w", err)
			}
		}
		id := source + ":" + work.RemoteID
		if _, err := tx.ExecContext(ctx, `INSERT INTO works(id,source,remote_id,kind,title,author_id,published_at,duration_ms,width,height,image_count,is_live,cover_url,extra,updated_at,available)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,COALESCE(?,1)) ON CONFLICT(id) DO UPDATE SET
kind=excluded.kind,title=excluded.title,author_id=excluded.author_id,published_at=excluded.published_at,
duration_ms=excluded.duration_ms,width=excluded.width,height=excluded.height,image_count=excluded.image_count,
is_live=excluded.is_live,cover_url=excluded.cover_url,extra=excluded.extra,updated_at=excluded.updated_at,available=COALESCE(?,works.available)`,
			id, source, work.RemoteID, work.Kind, work.Title, authorID, work.PublishedAt, work.DurationMS, work.Width, work.Height, work.ImageCount, work.IsLive, work.CoverURL, string(extra), now, work.Available, work.Available); err != nil {
			return fmt.Errorf("upsert work: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO relations(account_id,work_id,kind,seq,published_at,first_seen) VALUES(?,?,?,?,?,?)
ON CONFLICT(account_id,kind,work_id) DO UPDATE SET published_at=excluded.published_at`,
			page.AccountID, id, page.Kind, seq, work.PublishedAt, now); err != nil {
			return fmt.Errorf("upsert relation: %w", err)
		}
		seq--
	}
	phase, status := page.Phase, "running"
	if page.Complete {
		phase, status = "done", "paused"
	}
	headCursor, backfillCursor := "0", any(nil)
	var headSeq int64
	if page.Phase == "head" {
		headCursor = page.Cursor
		headSeq = upper + 1
	} else {
		backfillCursor = page.Cursor
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sync_state(account_id,kind,phase,status,head_cursor,backfill_cursor,head_seq,pages,items,updated_at)
VALUES(?,?,?,?,?,?,?,1,?,?) ON CONFLICT(account_id,kind) DO UPDATE SET
phase=excluded.phase,status=excluded.status,
head_cursor=CASE WHEN ?='head' THEN excluded.head_cursor ELSE sync_state.head_cursor END,
backfill_cursor=CASE WHEN ?='backfill' THEN excluded.backfill_cursor ELSE sync_state.backfill_cursor END,
head_seq=CASE WHEN ?='head' THEN excluded.head_seq ELSE sync_state.head_seq END,
pages=sync_state.pages+1,items=sync_state.items+excluded.items,updated_at=excluded.updated_at`,
		page.AccountID, page.Kind, phase, status, headCursor, backfillCursor, headSeq, len(page.Works), now, page.Phase, page.Phase, page.Phase); err != nil {
		return fmt.Errorf("advance page cursor: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit work page: %w", err)
	}
	return nil
}

func (s *Store) ListWorks(ctx context.Context, accountID, kind string, after *WorkCursor, limit int) ([]Work, *WorkCursor, error) {
	if limit < 1 || limit > 1000 {
		return nil, nil, fmt.Errorf("page limit must be between 1 and 1000")
	}
	query := `SELECT w.id,w.source,w.remote_id,w.kind,w.title,w.published_at,w.duration_ms,w.width,w.height,w.image_count,w.is_live,COALESCE(w.cover_url,''),COALESCE(w.cover_hash,''),w.available,w.extra,r.seq,
COALESCE(a.remote_id,''),COALESCE(a.nickname,''),COALESCE(a.avatar_url,'')
FROM relations r JOIN works w ON w.id=r.work_id LEFT JOIN authors a ON a.id=w.author_id
WHERE r.account_id=? AND r.kind=?`
	args := []any{accountID, kind}
	if after != nil {
		query += ` AND (r.seq<? OR (r.seq=? AND r.work_id>?))`
		args = append(args, after.Seq, after.Seq, after.WorkID)
	}
	query += ` ORDER BY r.seq DESC,r.work_id LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list works: %w", err)
	}
	defer rows.Close()
	works := []Work{}
	for rows.Next() {
		var work Work
		var author Author
		var extra string
		if err := rows.Scan(&work.ID, &work.Source, &work.RemoteID, &work.Kind, &work.Title, &work.PublishedAt, &work.DurationMS, &work.Width, &work.Height, &work.ImageCount, &work.IsLive, &work.CoverURL, &work.CoverHash, &work.Available, &extra, &work.Seq, &author.RemoteID, &author.Nickname, &author.AvatarURL); err != nil {
			return nil, nil, fmt.Errorf("read work: %w", err)
		}
		work.Extra = json.RawMessage(extra)
		if author.RemoteID != "" {
			work.Author = &author
		}
		works = append(works, work)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate works: %w", err)
	}
	var next *WorkCursor
	if len(works) > limit {
		works = works[:limit]
		last := works[len(works)-1]
		next = &WorkCursor{Seq: last.Seq, WorkID: last.ID}
	}
	return works, next, nil
}

func (s *Store) CountWorks(ctx context.Context, accountID, kind string) (int64, error) {
	var count int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM relations WHERE account_id=? AND kind=?`, accountID, kind).Scan(&count); err != nil {
		return 0, fmt.Errorf("count works: %w", err)
	}
	return count, nil
}

func (s *Store) RemoveRelation(ctx context.Context, accountID, kind, workID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM relations WHERE account_id=? AND kind=? AND work_id=?`, accountID, kind, workID); err != nil {
		return fmt.Errorf("remove relation: %w", err)
	}
	return nil
}

func (s *Store) CleanupOrphanWorks(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM works WHERE NOT EXISTS (SELECT 1 FROM relations WHERE work_id=works.id)`)
	if err != nil {
		return 0, fmt.Errorf("clean orphan works: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count orphan works: %w", err)
	}
	return count, nil
}
