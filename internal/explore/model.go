package explore

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/store"
)

const SchemaVersion = 2
const AnalysisVersion = "native-literal-baseline-v2"
const InputVersion = "native-library-transaction-v1"

//go:embed rules.json
var ruleJSON []byte

type Rule struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Sources []string `json:"sources"`
	Clues   []string `json:"clues"`
	Reason  string   `json:"reason"`
}

var Rules []Rule
var RuleHash string

func init() {
	if err := json.Unmarshal(ruleJSON, &Rules); err != nil {
		panic(err)
	}
	RuleHash = hash([]any{AnalysisVersion, Rules})
}
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

type matcher struct {
	clue  string
	lower string
	re    *regexp.Regexp
}

func matchers(rule Rule) []matcher {
	out := []matcher{}
	for _, c := range rule.Clues {
		m := matcher{clue: c, lower: strings.ToLower(c)}
		if asciiClue.MatchString(m.lower) {
			m.re = regexp.MustCompile(`(^|[^a-z0-9])` + regexp.QuoteMeta(m.lower) + `($|[^a-z0-9])`)
		}
		out = append(out, m)
	}
	return out
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
func analyzeSource(ctx context.Context, source string, rows []store.ExploreRecord) (SourceSnapshot, error) {
	s := SourceSnapshot{Source: source, Works: []Work{}, Topics: []Topic{}, TopicMembership: []Membership{}, Edges: []Edge{}, Coverage: Coverage{Source: source}, TimeCapabilities: TimeCapabilities{Views: map[string]bool{"current": true, "source_time": false, "observed_time": false}}}
	texts := map[string]string{}
	for i, r := range rows {
		if i%128 == 0 && ctx.Err() != nil {
			return s, ctx.Err()
		}
		if r.Source != source {
			continue
		}
		w := Work{ID: r.ID, Source: source, Title: r.Title, MediaKind: "video", TextAvailability: "ok", Times: []string{}, Author: Author{Name: r.Author}}
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
		texts[w.ID] = Clean(w.Title)
	}
	assigned := map[string]bool{}
	for _, rule := range Rules {
		if ctx.Err() != nil {
			return s, ctx.Err()
		}
		supported := false
		for _, src := range rule.Sources {
			if src == source {
				supported = true
			}
		}
		if !supported {
			continue
		}
		ms := matchers(rule)
		matches := []Membership{}
		ids := []string{}
		for i, w := range s.Works {
			if i%128 == 0 && ctx.Err() != nil {
				return s, ctx.Err()
			}
			hits := []string{}
			for _, m := range ms {
				t := texts[w.ID]
				if m.re != nil && m.re.MatchString(t) || m.re == nil && utf8.RuneCountInString(m.lower) >= 2 && strings.Contains(t, m.lower) {
					hits = append(hits, m.clue)
				}
			}
			if len(hits) == 0 {
				continue
			}
			if len(hits) > 4 {
				hits = hits[:4]
			}
			ev := []Evidence{}
			for _, h := range hits {
				ev = append(ev, Evidence{Field: "title", Excerpt: h})
			}
			ids = append(ids, w.ID)
			matches = append(matches, Membership{WorkID: w.ID, TopicID: rule.ID, Method: "literal_contains", Confidence: "medium", Reason: "标题含：" + strings.Join(hits, "、"), Evidence: ev})
		}
		if len(matches) < 3 {
			continue
		}
		s.Topics = append(s.Topics, Topic{ID: rule.ID, Label: rule.Label, Kind: "topic", WorkCount: len(ids), WorkIDs: ids})
		s.TopicMembership = append(s.TopicMembership, matches...)
		for _, id := range ids {
			assigned[id] = true
		}
	}
	for _, b := range []struct{ id, label, reason string }{{"t_uncategorized", "未归类", "未命中主题规则"}, {"t_no_text", "无文案", "无文案，无法规则归类"}} {
		ids := []string{}
		for _, w := range s.Works {
			match := b.id == "t_no_text" && w.TextAvailability == "none" || b.id == "t_uncategorized" && w.TextAvailability != "none" && !assigned[w.ID]
			if !match {
				continue
			}
			ids = append(ids, w.ID)
			s.TopicMembership = append(s.TopicMembership, Membership{WorkID: w.ID, TopicID: b.id, Method: "fallback", Confidence: "low", Reason: b.reason, Evidence: []Evidence{}})
		}
		s.Topics = append(s.Topics, Topic{ID: b.id, Label: b.label, Kind: "bucket", WorkCount: len(ids), WorkIDs: ids})
		if b.id == "t_no_text" {
			s.Coverage.WorksNoText = len(ids)
		} else {
			s.Coverage.UncategorizedWorks = len(ids)
		}
	}

	byWork := map[string][]Membership{}
	for _, m := range s.TopicMembership {
		byWork[m.WorkID] = append(byWork[m.WorkID], m)
	}
	longest := func(m Membership) int {
		v := 0
		for _, e := range m.Evidence {
			n := len(utf16.Encode([]rune(e.Excerpt)))
			if n > v {
				v = n
			}
		}
		return v
	}
	for i := range s.Works {
		list := byWork[s.Works[i].ID]
		sort.Slice(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if len(a.Evidence) != len(b.Evidence) {
				return len(a.Evidence) > len(b.Evidence)
			}
			if longest(a) != longest(b) {
				return longest(a) > longest(b)
			}
			return a.TopicID < b.TopicID
		})
		if len(list) > 0 {
			s.Works[i].AnchorTopicID = list[0].TopicID
		}
	}
	for i, a := range s.Topics {
		if a.Kind != "topic" {
			continue
		}
		set := map[string]bool{}
		for _, id := range a.WorkIDs {
			set[id] = true
		}
		for _, b := range s.Topics[i+1:] {
			if b.Kind != "topic" {
				continue
			}
			co := 0
			for _, id := range b.WorkIDs {
				if set[id] {
					co++
				}
			}
			j := float64(co) / float64(a.WorkCount+b.WorkCount-co)
			if co >= 2 && j >= .03 {
				s.Edges = append(s.Edges, Edge{Source: a.ID, Target: b.ID, CoCount: co, Jaccard: j, Method: "co_membership_jaccard"})
			}
		}
	}
	sort.SliceStable(s.Edges, func(i, j int) bool { return s.Edges[i].Jaccard > s.Edges[j].Jaccard })
	if len(s.Edges) > 40 {
		s.Edges = s.Edges[:40]
	}
	s.Coverage.WorksTotal = len(s.Works)
	s.Coverage.WorksInTopics = len(assigned)
	return s, nil
}
