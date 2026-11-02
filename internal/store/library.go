package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// LibraryQuery selects one page of a list in the library.
type LibraryQuery struct {
	AccountID string
	Source    string
	Kind      string // like | favorite | music
	Type      string // all | video | gallery
	Query     string
	Sort      string // published | recent
	Offset    int
	Limit     int
}

// LibraryItem is one row of the library grid.
type LibraryItem struct {
	RemoteID    string
	Kind        string
	Title       string
	Author      string
	CoverURL    string
	PublishedAt int64
	DurationMS  int
	ImageCount  int
}

func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(q)) + "%"
}

// MediaRecord is the stored metadata needed to resolve one item.
type MediaRecord struct {
	RemoteID, Kind, Title, Author string
	CoverURL                      string
	PublishedAt                   int64
	Extra                         []byte
	HasAvatar                     bool
}

// MediaRecord returns a stored work or music by remote id.
func (s *Store) MediaRecord(ctx context.Context, kind, remoteID string) (MediaRecord, error) {
	var r MediaRecord
	if kind == "music" {
		err := s.db.QueryRowContext(ctx, `SELECT remote_id,'music',title,author,cover_url,0,extra FROM musics WHERE remote_id=? LIMIT 1`, remoteID).Scan(&r.RemoteID, &r.Kind, &r.Title, &r.Author, &r.CoverURL, &r.PublishedAt, &r.Extra)
		return r, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT w.remote_id,w.kind,w.title,COALESCE(a.nickname,''),COALESCE(w.cover_url,''),w.published_at,w.extra,COALESCE(a.avatar_url,'')<>'' FROM works w LEFT JOIN authors a ON a.id=w.author_id WHERE w.remote_id=? LIMIT 1`, remoteID).Scan(&r.RemoteID, &r.Kind, &r.Title, &r.Author, &r.CoverURL, &r.PublishedAt, &r.Extra, &r.HasAvatar)
	return r, err
}

// FillAuthorAvatar stores an avatar for a work's author that has none yet.
func (s *Store) FillAuthorAvatar(ctx context.Context, workRemoteID, avatar string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE authors SET avatar_url=? WHERE id=(SELECT author_id FROM works WHERE remote_id=?) AND COALESCE(avatar_url,'')=''`, avatar, workRemoteID)
	return err
}

// Library returns one page of a list and the number of matching items.
func (s *Store) Library(ctx context.Context, q LibraryQuery) ([]LibraryItem, int, error) {
	if q.Limit < 1 || q.Limit > 500 {
		q.Limit = 60
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if where, args, order, ok := relationsOnly(q); ok {
		return s.libraryByRelations(ctx, where, args, order, q)
	}
	from, filter, order, args, err := listFilter(q)
	if err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.read.QueryRowContext(ctx, `SELECT COUNT(DISTINCT w.id) `+from+filter, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count library: %w", err)
	}
	columns := `w.remote_id,w.kind,w.title,COALESCE(a.nickname,''),COALESCE(w.cover_url,''),w.published_at,w.duration_ms,w.image_count`
	if q.Kind == "music" {
		columns = `w.remote_id,'music',w.title,w.author,COALESCE(w.cover_url,''),0,w.duration_ms,0`
	}
	rows, err := s.read.QueryContext(ctx, `SELECT `+columns+` `+from+filter+` GROUP BY w.id ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list library: %w", err)
	}
	defer rows.Close()
	items := []LibraryItem{}
	for rows.Next() {
		var it LibraryItem
		if err := rows.Scan(&it.RemoteID, &it.Kind, &it.Title, &it.Author, &it.CoverURL, &it.PublishedAt, &it.DurationMS, &it.ImageCount); err != nil {
			return nil, 0, fmt.Errorf("read library item: %w", err)
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// relationsOnly reports whether a list can be read in its account's relation order: one
// account's works are unique, so nothing needs grouping, and the rows walk an index in
// order. Without a type or search filter the works are joined only for the rows returned,
// so pages far down cost about as little as the first.
func relationsOnly(q LibraryQuery) (where string, args []any, order string, ok bool) {
	if q.AccountID == "" || (q.Kind != "like" && q.Kind != "favorite") {
		return "", nil, "", false
	}
	where, args = "r.account_id=? AND r.kind=?", []any{q.AccountID, q.Kind}
	if q.Type == "video" || q.Type == "gallery" {
		where, args = where+" AND w.kind=?", append(args, q.Type)
	}
	if strings.TrimSpace(q.Query) != "" {
		p := likePattern(q.Query)
		where, args = where+` AND (w.title LIKE ? ESCAPE '\' OR a.nickname LIKE ? ESCAPE '\')`, append(args, p, p)
	}
	if q.Source != "" {
		// Work ids start with their source.
		where, args = where+" AND substr(r.work_id,1,length(?)+1)=?||':'", append(args, q.Source, q.Source)
	}
	// Relations keep each work's publish time, which never changes, so the order matches
	// sorting by the work itself.
	order = "r.seq DESC"
	if q.Sort == "published" {
		order = "r.published_at DESC, r.work_id"
	}
	return where, args, order, true
}

// relationsFrom joins the works and authors only when a filter needs them.
func relationsFrom(q LibraryQuery) string {
	if q.Type == "video" || q.Type == "gallery" || strings.TrimSpace(q.Query) != "" {
		return `FROM relations r JOIN works w ON w.id=r.work_id LEFT JOIN authors a ON a.id=w.author_id`
	}
	return `FROM relations r`
}

func (s *Store) libraryByRelations(ctx context.Context, where string, args []any, order string, q LibraryQuery) ([]LibraryItem, int, error) {
	from := relationsFrom(q)
	var total int
	if err := s.read.QueryRowContext(ctx, `SELECT COUNT(*) `+from+` WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count library: %w", err)
	}
	rows, err := s.read.QueryContext(ctx, `WITH r AS (SELECT r.work_id, r.published_at, r.seq `+from+` WHERE `+where+` ORDER BY `+order+` LIMIT ? OFFSET ?)
SELECT w.remote_id,w.kind,w.title,COALESCE(a.nickname,''),COALESCE(w.cover_url,''),w.published_at,w.duration_ms,w.image_count
FROM r JOIN works w ON w.id=r.work_id LEFT JOIN authors a ON a.id=w.author_id ORDER BY `+order, append(args, q.Limit, q.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list library: %w", err)
	}
	defer rows.Close()
	items := []LibraryItem{}
	for rows.Next() {
		var it LibraryItem
		if err := rows.Scan(&it.RemoteID, &it.Kind, &it.Title, &it.Author, &it.CoverURL, &it.PublishedAt, &it.DurationMS, &it.ImageCount); err != nil {
			return nil, 0, fmt.Errorf("read library item: %w", err)
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

// listFilter builds the source, conditions and order of one list, shared by every
// query over it so they always agree on what the list holds.
func listFilter(q LibraryQuery) (from, filter, order string, args []any, err error) {
	where := []string{}
	args = []any{}
	single := q.AccountID != ""
	if q.Kind == "music" {
		from = `FROM music_relations r JOIN musics w ON w.id=r.music_id`
		if single {
			where, args = append(where, "r.account_id=?"), append(args, q.AccountID)
		}
		if q.Source != "" {
			where, args = append(where, "w.source=?"), append(args, q.Source)
		}
		if strings.TrimSpace(q.Query) != "" {
			p := likePattern(q.Query)
			where, args = append(where, `(w.title LIKE ? ESCAPE '\' OR w.author LIKE ? ESCAPE '\')`), append(args, p, p)
		}
		order = "MAX(r.first_seen) DESC, MAX(r.seq) DESC"
		if single {
			order = "MAX(r.seq) DESC"
		}
	} else {
		if q.Kind != "like" && q.Kind != "favorite" {
			return "", "", "", nil, fmt.Errorf("invalid list kind")
		}
		from = `FROM relations r JOIN works w ON w.id=r.work_id LEFT JOIN authors a ON a.id=w.author_id`
		where, args = append(where, "r.kind=?"), append(args, q.Kind)
		if single {
			where, args = append(where, "r.account_id=?"), append(args, q.AccountID)
		}
		if q.Source != "" {
			where, args = append(where, "w.source=?"), append(args, q.Source)
		}
		if q.Type == "video" || q.Type == "gallery" {
			where, args = append(where, "w.kind=?"), append(args, q.Type)
		}
		if strings.TrimSpace(q.Query) != "" {
			p := likePattern(q.Query)
			where, args = append(where, `(w.title LIKE ? ESCAPE '\' OR a.nickname LIKE ? ESCAPE '\')`), append(args, p, p)
		}
		switch {
		case q.Sort == "published":
			order = "w.published_at DESC, w.id"
		case single:
			order = "MAX(r.seq) DESC"
		default:
			order = "MAX(r.first_seen) DESC, MAX(r.seq) DESC"
		}
	}
	if len(where) > 0 {
		filter = " WHERE " + strings.Join(where, " AND ")
	}
	return from, filter, order, args, nil
}

// TimelineMonth is one month of a list sorted by publish time.
type TimelineMonth struct {
	Year  int
	Month int // 1-12; 0 with Year 0 for items without a publish time
	Count int
	Start int // offset of the month's first item in the list
}

// Timeline returns the months of a list, newest first, with how many items each
// holds and where it starts. Only lists sorted by publish time have one.
func (s *Store) Timeline(ctx context.Context, q LibraryQuery) ([]TimelineMonth, error) {
	months := []TimelineMonth{}
	if q.Kind == "music" || q.Sort != "published" {
		return months, nil
	}
	// Publish times newest first; months are then runs of equal local year and month.
	var query string
	var args []any
	if where, fast, _, ok := relationsOnly(q); ok {
		query, args = `SELECT r.published_at `+relationsFrom(q)+` WHERE `+where+` ORDER BY r.published_at DESC`, fast
	} else {
		from, filter, _, slow, err := listFilter(q)
		if err != nil {
			return nil, err
		}
		query, args = `SELECT p FROM (SELECT w.published_at p `+from+filter+` GROUP BY w.id) ORDER BY p DESC`, slow
	}
	rows, err := s.read.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("timeline: %w", err)
	}
	defer rows.Close()
	start := 0
	for rows.Next() {
		var at int64
		if err := rows.Scan(&at); err != nil {
			return nil, fmt.Errorf("read timeline: %w", err)
		}
		// Months follow the local calendar, like the dates the app shows.
		year, month := 0, 0
		if at > 0 {
			d := time.Unix(at, 0)
			year, month = d.Year(), int(d.Month())
		}
		if n := len(months); n == 0 || months[n-1].Year != year || months[n-1].Month != month {
			months = append(months, TimelineMonth{Year: year, Month: month, Start: start})
		}
		months[len(months)-1].Count++
		start++
	}
	return months, rows.Err()
}

// LibraryCounts returns how many distinct items each list holds in a scope.
func (s *Store) LibraryCounts(ctx context.Context, accountID, source string) (map[string]int, error) {
	counts := map[string]int{"like": 0, "favorite": 0, "music": 0}
	for _, kind := range []string{"like", "favorite", "music"} {
		_, total, err := s.Library(ctx, LibraryQuery{AccountID: accountID, Source: source, Kind: kind, Type: "all", Limit: 1})
		if err != nil {
			return nil, err
		}
		counts[kind] = total
	}
	return counts, nil
}
