package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SyncRecord is the persisted progress of one list of one account.
type SyncRecord struct {
	Kind           string
	Phase          string
	Status         string
	BackfillCursor string
	Pages          int
	Items          int
	RemoteTotal    *int
	ErrorCode      string
	RetryAt        *int64
	Failures       int
	LastFullAt     *int64
	UpdatedAt      int64
}

// SyncRecords returns every list state of an account.
func (s *Store) SyncRecords(ctx context.Context, accountID string) ([]SyncRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind,phase,status,COALESCE(backfill_cursor,''),pages,items,remote_total,COALESCE(error_code,''),retry_at,failures,last_full_at,updated_at FROM sync_state WHERE account_id=?`, accountID)
	if err != nil {
		return nil, fmt.Errorf("read sync records: %w", err)
	}
	defer rows.Close()
	var out []SyncRecord
	for rows.Next() {
		var r SyncRecord
		var total, retry, full sql.NullInt64
		if err := rows.Scan(&r.Kind, &r.Phase, &r.Status, &r.BackfillCursor, &r.Pages, &r.Items, &total, &r.ErrorCode, &retry, &r.Failures, &full, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan sync record: %w", err)
		}
		if total.Valid {
			v := int(total.Int64)
			r.RemoteTotal = &v
		}
		if retry.Valid {
			r.RetryAt = &retry.Int64
		}
		if full.Valid {
			r.LastFullAt = &full.Int64
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PutSyncRecord stores a list state; items is recounted from the library.
func (s *Store) PutSyncRecord(ctx context.Context, accountID string, r SyncRecord) error {
	var backfill any
	if r.BackfillCursor != "" {
		backfill = r.BackfillCursor
	}
	var code any
	if r.ErrorCode != "" {
		code = r.ErrorCode
	}
	count := `SELECT COUNT(*) FROM relations WHERE account_id=?1 AND kind=?2`
	if r.Kind == "music" {
		count = `SELECT COUNT(*) FROM music_relations WHERE account_id=?1 AND ?2='music'`
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO sync_state(account_id,kind,phase,status,backfill_cursor,pages,items,remote_total,error_code,retry_at,failures,last_full_at,updated_at)
VALUES(?1,?2,?3,?4,?5,?6,(`+count+`),?7,?8,?9,?10,?11,?12) ON CONFLICT(account_id,kind) DO UPDATE SET
phase=excluded.phase,status=excluded.status,backfill_cursor=excluded.backfill_cursor,pages=excluded.pages,items=excluded.items,
remote_total=excluded.remote_total,error_code=excluded.error_code,retry_at=excluded.retry_at,failures=excluded.failures,
last_full_at=excluded.last_full_at,updated_at=excluded.updated_at`,
		accountID, r.Kind, r.Phase, r.Status, backfill, r.Pages, r.RemoteTotal, code, r.RetryAt, r.Failures, r.LastFullAt, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("write sync record: %w", err)
	}
	return nil
}

// KnownWorks reports which remote ids are already in an account's list.
func (s *Store) KnownWorks(ctx context.Context, accountID, kind string, remoteIDs []string) (map[string]bool, error) {
	known := map[string]bool{}
	if len(remoteIDs) == 0 {
		return known, nil
	}
	var source string
	if err := s.db.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, accountID).Scan(&source); err != nil {
		return nil, fmt.Errorf("read known account: %w", err)
	}
	table, column, kindFilter := "relations", "work_id", " AND kind=?"
	if kind == "music" {
		table, column, kindFilter = "music_relations", "music_id", ""
	}
	args := []any{accountID}
	if kindFilter != "" {
		args = append(args, kind)
	}
	marks := make([]string, len(remoteIDs))
	for i, id := range remoteIDs {
		marks[i] = "?"
		args = append(args, source+":"+id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+column+` FROM `+table+` WHERE account_id=?`+kindFilter+` AND `+column+` IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("read known works: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		known[strings.TrimPrefix(id, source+":")] = true
	}
	return known, rows.Err()
}

// SaveWorks adds works to a list. Head pages go above every saved item,
// backfill pages below, so seq DESC is always the list's own order.
func (s *Store) SaveWorks(ctx context.Context, accountID, kind string, head bool, works []Work) (int, error) {
	if kind != "like" && kind != "favorite" && kind != "watch_later" {
		return 0, fmt.Errorf("invalid work relation kind")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin save works: %w", err)
	}
	defer tx.Rollback()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, accountID).Scan(&source); err != nil {
		return 0, fmt.Errorf("read account: %w", err)
	}
	var upper, lower int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0),COALESCE(MIN(seq),0) FROM relations WHERE account_id=? AND kind=?`, accountID, kind).Scan(&upper, &lower); err != nil {
		return 0, fmt.Errorf("read sequence bounds: %w", err)
	}
	seq := lower - 1
	if head {
		seq = upper + int64(len(works))
	}
	now := time.Now().Unix()
	added := 0
	for _, work := range works {
		if work.RemoteID == "" || (work.Kind != "video" && work.Kind != "gallery") {
			return 0, fmt.Errorf("work identity and kind are required")
		}
		extra := work.Extra
		if len(extra) == 0 || !json.Valid(extra) {
			extra = json.RawMessage(`{}`)
		}
		var authorID any
		if work.Author != nil && work.Author.RemoteID != "" {
			authorID = source + ":" + work.Author.RemoteID
			if _, err := tx.ExecContext(ctx, `INSERT INTO authors(id,source,remote_id,nickname,avatar_url,updated_at) VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET nickname=excluded.nickname,avatar_url=excluded.avatar_url,updated_at=excluded.updated_at`,
				authorID, source, work.Author.RemoteID, work.Author.Nickname, work.Author.AvatarURL, now); err != nil {
				return 0, fmt.Errorf("upsert author: %w", err)
			}
		}
		id := source + ":" + work.RemoteID
		if _, err := tx.ExecContext(ctx, `INSERT INTO works(id,source,remote_id,kind,title,author_id,published_at,duration_ms,width,height,image_count,is_live,cover_url,extra,updated_at,available)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,COALESCE(?,1)) ON CONFLICT(id) DO UPDATE SET
kind=excluded.kind,title=excluded.title,author_id=excluded.author_id,published_at=excluded.published_at,
duration_ms=excluded.duration_ms,width=excluded.width,height=excluded.height,image_count=excluded.image_count,
is_live=excluded.is_live,cover_url=excluded.cover_url,extra=excluded.extra,updated_at=excluded.updated_at,available=COALESCE(?,works.available)`,
			id, source, work.RemoteID, work.Kind, work.Title, authorID, work.PublishedAt, work.DurationMS, work.Width, work.Height, work.ImageCount, work.IsLive, work.CoverURL, string(extra), now, work.Available, work.Available); err != nil {
			return 0, fmt.Errorf("upsert work: %w", err)
		}
		relationSeq := seq
		if work.AddedAt > 0 {
			relationSeq = work.AddedAt
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO relations(account_id,work_id,kind,seq,published_at,first_seen) VALUES(?,?,?,?,?,?)
ON CONFLICT(account_id,kind,work_id) DO NOTHING`, accountID, id, kind, relationSeq, work.PublishedAt, now)
		if err != nil {
			return 0, fmt.Errorf("insert relation: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
		if source == "bilibili" {
			if _, err := tx.ExecContext(ctx, `UPDATE relations SET published_at=?,seq=CASE WHEN ?>0 THEN ? ELSE seq END WHERE account_id=? AND kind=? AND work_id=?`, work.PublishedAt, work.AddedAt, work.AddedAt, accountID, kind, id); err != nil {
				return 0, fmt.Errorf("update relation metadata: %w", err)
			}
		}
		seq--
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit save works: %w", err)
	}
	return added, nil
}

// Music is a saved sound.
type Music struct {
	RemoteID   string
	Title      string
	Author     string
	DurationMS int
	CoverURL   string
	Extra      json.RawMessage
}

// SaveMusics adds sounds to an account's music list, in list order.
func (s *Store) SaveMusics(ctx context.Context, accountID string, head bool, musics []Music) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin save musics: %w", err)
	}
	defer tx.Rollback()
	var source string
	if err := tx.QueryRowContext(ctx, `SELECT source FROM accounts WHERE id=?`, accountID).Scan(&source); err != nil {
		return 0, fmt.Errorf("read account: %w", err)
	}
	var upper, lower int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq),0),COALESCE(MIN(seq),0) FROM music_relations WHERE account_id=?`, accountID).Scan(&upper, &lower); err != nil {
		return 0, fmt.Errorf("read music bounds: %w", err)
	}
	seq := lower - 1
	if head {
		seq = upper + int64(len(musics))
	}
	now := time.Now().Unix()
	added := 0
	for _, m := range musics {
		if m.RemoteID == "" {
			return 0, fmt.Errorf("music identity is required")
		}
		extra := m.Extra
		if len(extra) == 0 || !json.Valid(extra) {
			extra = json.RawMessage(`{}`)
		}
		id := source + ":" + m.RemoteID
		if _, err := tx.ExecContext(ctx, `INSERT INTO musics(id,source,remote_id,title,author,duration_ms,cover_url,extra,updated_at) VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET title=excluded.title,author=excluded.author,duration_ms=excluded.duration_ms,cover_url=excluded.cover_url,extra=excluded.extra,updated_at=excluded.updated_at`,
			id, source, m.RemoteID, m.Title, m.Author, m.DurationMS, m.CoverURL, string(extra), now); err != nil {
			return 0, fmt.Errorf("upsert music: %w", err)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO music_relations(account_id,music_id,seq,first_seen) VALUES(?,?,?,?) ON CONFLICT(account_id,music_id) DO NOTHING`, accountID, id, seq, now)
		if err != nil {
			return 0, fmt.Errorf("insert music relation: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			added++
		}
		seq--
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit save musics: %w", err)
	}
	return added, nil
}

// CoverURLs returns the stored cover URLs of one work ("w"), music ("m"),
// gallery image ("g", id:index) or work author avatar ("a") by remote id, best first.
func (s *Store) CoverURLs(ctx context.Context, kind, remoteID string) ([]string, error) {
	if kind == "account" {
		var avatar string
		err := s.read.QueryRowContext(ctx, `SELECT COALESCE(avatar_url,'') FROM accounts WHERE id=?`, remoteID).Scan(&avatar)
		if err != nil || avatar == "" {
			return nil, err
		}
		if strings.HasPrefix(avatar, "http://") {
			avatar = "https://" + strings.TrimPrefix(avatar, "http://")
		}
		return []string{avatar}, nil
	}
	if kind == "a" {
		var avatar string
		err := s.read.QueryRowContext(ctx, `SELECT COALESCE(a.avatar_url,'') FROM works w JOIN authors a ON a.id=w.author_id WHERE w.remote_id=? LIMIT 1`, remoteID).Scan(&avatar)
		if err != nil || avatar == "" {
			return nil, err
		}
		return []string{avatar}, nil
	}
	table := "works"
	if kind == "m" {
		table = "musics"
	}
	imageIndex := -1
	if kind == "g" {
		parts := strings.Split(remoteID, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid gallery image")
		}
		var err error
		imageIndex, err = strconv.Atoi(parts[1])
		if err != nil || imageIndex < 0 {
			return nil, fmt.Errorf("invalid gallery image")
		}
		remoteID = parts[0]
	}
	var cover, extra string
	err := s.read.QueryRowContext(ctx, `SELECT COALESCE(cover_url,''),extra FROM `+table+` WHERE remote_id=? LIMIT 1`, remoteID).Scan(&cover, &extra)
	if err != nil {
		return nil, err
	}
	var meta struct {
		Covers []string `json:"covers"`
		Images []string `json:"images"`
	}
	_ = json.Unmarshal([]byte(extra), &meta)
	if kind == "g" {
		if imageIndex >= len(meta.Images) {
			return nil, nil
		}
		return []string{meta.Images[imageIndex]}, nil
	}
	out := []string{}
	seen := map[string]bool{}
	if len(meta.Images) > 1 {
		meta.Images = meta.Images[:1]
	}
	for _, u := range append(append([]string{cover}, meta.Covers...), meta.Images...) {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out, nil
}
