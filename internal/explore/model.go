package explore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/store"
)

const SchemaVersion = 3
const AnalysisVersion = "native-sparse-neighbors-v5-weak-gse-1.3.1-zh-s"
const InputVersion = "native-library-publication-v2"

// RuleHash versions the local analyzer; no topic keyword rules are loaded.
var RuleHash = hash(AnalysisVersion)

func hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type Scope = store.ExploreScope
type Author struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarURL"`
}
type MonthKeys struct {
	SourceTime   *string `json:"source_time"`
	ObservedTime *string `json:"observed_time"`
}
type Work struct {
	PublishedAt      int64     `json:"publishedAt"`
	AnchorTopicID    string    `json:"anchorTopicId"`
	ID               string    `json:"id"`
	Source           string    `json:"source"`
	Title            string    `json:"title"`
	MediaKind        string    `json:"mediaKind"`
	TextAvailability string    `json:"textAvailability"`
	Cover            string    `json:"cover"`
	Author           Author    `json:"author"`
	MonthKeys        MonthKeys `json:"monthKeys"`
	Times            []string  `json:"times"`
}
type Evidence struct {
	Field   string `json:"field"`
	Excerpt string `json:"excerpt"`
}
type Membership struct {
	WorkID     string     `json:"workId"`
	TopicID    string     `json:"topicId"`
	Method     string     `json:"method"`
	Confidence string     `json:"confidence"`
	Reason     string     `json:"reason"`
	Evidence   []Evidence `json:"evidence"`
}
type Topic struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	WorkCount int      `json:"workCount"`
	WorkIDs   []string `json:"workIds"`
}
type Edge struct {
	Source  string  `json:"source"`
	Target  string  `json:"target"`
	CoCount int     `json:"coCount"`
	Jaccard float64 `json:"jaccard"`
	Method  string  `json:"method"`
}
type Coverage struct {
	Source             string `json:"source"`
	WorksTotal         int    `json:"worksTotal"`
	WorksInTopics      int    `json:"worksInTopics"`
	UncategorizedWorks int    `json:"uncategorizedWorks"`
	WorksNoText        int    `json:"worksNoText"`
	AuthorNames        int    `json:"authorNames"`
	AuthorIDs          int    `json:"authorIds"`
	AvatarRefs         int    `json:"avatarRefs"`
	CoverRefs          int    `json:"coverRefs"`
}
type TimeCapabilities struct {
	Views map[string]bool `json:"views"`
}
type SourceSnapshot struct {
	Source           string           `json:"source"`
	Works            []Work           `json:"works"`
	Topics           []Topic          `json:"topics"`
	TopicMembership  []Membership     `json:"topicMembership"`
	Edges            []Edge           `json:"edges"`
	Neighbors        []WorkNeighbor   `json:"neighbors"`
	Coverage         Coverage         `json:"coverage"`
	TimeCapabilities TimeCapabilities `json:"timeCapabilities"`
}
type Meta struct {
	Mode        string     `json:"mode"`
	Input       string     `json:"input"`
	Coverage    []Coverage `json:"coverage"`
	Persistence string     `json:"persistence"`
	Consistency string     `json:"consistency"`
}
type Snapshot struct {
	SchemaVersion   int              `json:"schemaVersion"`
	AnalysisVersion string           `json:"analysisVersion"`
	InputVersion    string           `json:"inputVersion"`
	SnapshotID      string           `json:"snapshotId"`
	InputRevision   string           `json:"inputRevision"`
	RuleHash        string           `json:"ruleHash"`
	SnapshotKind    string           `json:"snapshotKind"`
	CreatedAt       string           `json:"createdAt"`
	Scope           Scope            `json:"scope"`
	Meta            Meta             `json:"meta"`
	Scopes          []SourceSnapshot `json:"scopes"`
}

const ws = `\s\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}`

var urlPattern = regexp.MustCompile(`(?i)https?://[^` + ws + `]+|www\.[^` + ws + `]+`)
var mentionPattern = regexp.MustCompile(`@[^` + ws + `#，。！？,!?：:]+`)
var asciiClue = regexp.MustCompile(`^[a-z0-9_+#.\-]+$`)

func Clean(s string) string {
	s = urlPattern.ReplaceAllString(s, " ")
	s = mentionPattern.ReplaceAllString(s, " ")
	return strings.ToLower(strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' }), " "))
}
func ClueMatch(text, clue string) bool {
	c := strings.ToLower(clue)
	if asciiClue.MatchString(c) {
		return regexp.MustCompile(`(^|[^a-z0-9])` + regexp.QuoteMeta(c) + `($|[^a-z0-9])`).MatchString(text)
	}
	return utf8.RuneCountInString(c) >= 2 && strings.Contains(text, c)
}

func Revision(scope Scope, rows []store.ExploreRecord) string {
	copyRows := append([]store.ExploreRecord{}, rows...)
	sort.Slice(copyRows, func(i, j int) bool {
		return copyRows[i].Source+":"+copyRows[i].ID < copyRows[j].Source+":"+copyRows[j].ID
	})
	return hash([]any{InputVersion, scope, copyRows})
}
func NativeAsset(kind, id string) string { return covers.URL(kind, id) + "?cacheOnly=1" }
func Analyze(ctx context.Context, scope Scope, rows []store.ExploreRecord) (*Snapshot, error) {
	input := append([]store.ExploreRecord{}, rows...)
	sort.Slice(input, func(i, j int) bool { a, b := input[i], input[j]; return a.Source+":"+a.ID < b.Source+":"+b.ID })
	s := &Snapshot{SchemaVersion: SchemaVersion, AnalysisVersion: AnalysisVersion, InputVersion: InputVersion, InputRevision: Revision(scope, input), RuleHash: RuleHash, SnapshotKind: "baseline", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Scope: scope, Scopes: []SourceSnapshot{}, Meta: Meta{Mode: "real", Input: "native Store.ExploreInput", Persistence: "native atomic analysis cache", Consistency: "single read-only WAL transaction", Coverage: []Coverage{}}}
	s.SnapshotID = "current-" + s.InputRevision[:20] + "-" + RuleHash[:8]
	for _, source := range []string{"douyin", "bilibili"} {
		if scope.Source != "" && scope.Source != source {
			continue
		}
		sc, err := analyzeSource(ctx, source, input)
		if err != nil {
			return nil, err
		}
		s.Scopes = append(s.Scopes, sc)
		s.Meta.Coverage = append(s.Meta.Coverage, sc.Coverage)
	}
	return s, ctx.Err()
}
func publicationMonth(at int64) *string {
	if at <= 0 || at >= 253402300800 {
		return nil
	}
	// Match the existing library timeline's local calendar months.
	month := time.Unix(at, 0).In(time.Local).Format("2006-01")
	return &month
}

func analyzeSource(ctx context.Context, source string, rows []store.ExploreRecord) (SourceSnapshot, error) {
	s := SourceSnapshot{Source: source, Works: []Work{}, Topics: []Topic{}, TopicMembership: []Membership{}, Edges: []Edge{}, Neighbors: []WorkNeighbor{}, Coverage: Coverage{Source: source}, TimeCapabilities: TimeCapabilities{Views: map[string]bool{"current": true, "source_time": false, "observed_time": false}}}
	for i, r := range rows {
		if i%128 == 0 && ctx.Err() != nil {
			return s, ctx.Err()
		}
		if r.Source != source {
			continue
		}
		w := Work{ID: r.ID, Source: source, Title: r.Title, MediaKind: "video", TextAvailability: "ok", Times: []string{}, Author: Author{Name: r.Author}}
		w.MonthKeys.SourceTime = publicationMonth(r.PublishedAt)
		if w.MonthKeys.SourceTime != nil {
			w.PublishedAt = r.PublishedAt
			s.TimeCapabilities.Views["source_time"] = true
		}
		if r.Kind == "gallery" {
			w.MediaKind = "image"
		}
		if strings.TrimSpace(r.Title) == "" {
			w.TextAvailability = "none"
		}
		if r.CoverURL != "" {
			w.Cover = NativeAsset("w", r.ID)
			s.Coverage.CoverRefs++
		}
		if r.AuthorID != "" {
			w.Author.ID = source + ":" + r.AuthorID
			s.Coverage.AuthorIDs++
		}
		if r.AvatarURL != "" {
			w.Author.AvatarURL = NativeAsset("a", r.ID)
			s.Coverage.AvatarRefs++
		}
		if r.Author != "" {
			s.Coverage.AuthorNames++
		}
		s.Works = append(s.Works, w)
	}
	if err := associateWords(ctx, &s); err != nil {
		return s, err
	}
	s.Coverage.WorksTotal = len(s.Works)
	return s, nil
}
