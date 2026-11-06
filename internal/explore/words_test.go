package explore

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/wangxu-dev/hoard/internal/store"
)

func TestCanonicalWordsCollapseTagPhraseAndTokenPieces(t *testing.T) {
	tests := []struct {
		name string
		doc  wordDoc
		want []string
	}{
		{"same surface across all provenance", wordDoc{base: map[string]int{"摄影": 9, "旅行": 1}, tags: []string{"摄影", "摄影"}, phrases: []string{"摄影"}}, []string{"摄影", "旅行"}},
		{"compound and components are one family", wordDoc{base: map[string]int{"海水": 1, "造景": 1, "灯光": 1}, tags: []string{"海水造景"}, phrases: []string{"海水造景"}}, []string{"海水造景", "灯光"}},
		{"nested compounds", wordDoc{base: map[string]int{"海水": 1}, tags: []string{"海水造景", "海水造景教程"}, phrases: []string{"海水造景"}}, []string{"海水造景教程"}},
		{"ASCII containment observes word boundaries", wordDoc{base: map[string]int{"car": 1, "scarlet": 1}}, []string{"car", "scarlet"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalWords(tt.doc); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSparseWeightsDoNotCountSameEvidenceTwice(t *testing.T) {
	canonical := []wordDoc{
		{base: map[string]int{"海水造景": 1, "灯光": 1, "苹果": 1}},
		{base: map[string]int{"海水造景": 1, "灯光": 1, "香蕉": 1}},
	}
	duplicated := []wordDoc{
		{base: map[string]int{"海水": 8, "造景": 8, "灯光": 8, "苹果": 1}, tags: []string{"海水造景", "海水造景"}, phrases: []string{"海水造景"}},
		{base: map[string]int{"海水": 8, "造景": 8, "灯光": 8, "香蕉": 1}, tags: []string{"海水造景"}, phrases: []string{"海水造景", "海水造景"}},
	}
	a, err := sparseWordGraph(context.Background(), canonical)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sparseWordGraph(context.Background(), duplicated)
	if err != nil {
		t.Fatal(err)
	}
	if len(a[0]) != 1 || !reflect.DeepEqual(a, b) {
		t.Fatalf("duplicate provenance changed score/evidence: canonical=%+v duplicated=%+v", a, b)
	}
	idf := 1 + math.Log(3.0/2)
	want := 2 / (2 + idf*idf)
	if math.Abs(b[0][0].score-want) > 1e-12 {
		t.Fatalf("score %.15f != canonical %.15f", b[0][0].score, want)
	}
}

func TestSparseWordsRejectOneEvidenceFamily(t *testing.T) {
	d := wordDoc{base: map[string]int{"海水": 1, "造景": 1}, tags: []string{"海水造景"}, phrases: []string{"海水造景"}}
	g, err := sparseWordGraph(context.Background(), []wordDoc{d, d, {base: map[string]int{}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range g {
		if len(ns) > 0 {
			t.Fatal("one evidence family or empty text invented an edge")
		}
	}
}

func TestSparseWordsPostingAndNeighborBounds(t *testing.T) {
	d := wordDoc{base: map[string]int{"alpha": 1, "beta": 1}}
	crowded := make([]wordDoc, wordPostingLimit+1)
	for i := range crowded {
		crowded[i] = d
	}
	g, err := sparseWordGraph(context.Background(), crowded)
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range g {
		if len(ns) > 0 {
			t.Fatal("expanded a high-DF posting")
		}
	}
	g, err = sparseWordGraph(context.Background(), crowded[:30])
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range g {
		if len(ns) != wordTopK {
			t.Fatal("top-k neighbor budget changed")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = sparseWordGraph(ctx, crowded); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestNativeSparseTopicsUseOnlyRealWorksAndLocalEvidence(t *testing.T) {
	rows := []store.ExploreRecord{}
	for _, source := range []string{"douyin", "bilibili"} {
		rows = append(rows,
			store.ExploreRecord{Source: source, ID: "a", Title: "#海水造景 #灯光"},
			store.ExploreRecord{Source: source, ID: "b", Title: "#海水造景 #灯光"},
			store.ExploreRecord{Source: source, ID: "c", Title: "#海水造景"},
			store.ExploreRecord{Source: source, ID: "d", Title: ""},
		)
	}
	s, err := Analyze(context.Background(), Scope{}, rows)
	if err != nil {
		t.Fatal(err)
	}
	if err = Validate(s); err != nil {
		t.Fatal(err)
	}
	seenTopics := map[string]bool{}
	for _, sc := range s.Scopes {
		if len(sc.Works) != 4 || sc.Coverage.WorksInTopics != 2 || sc.Coverage.UncategorizedWorks != 1 || sc.Coverage.WorksNoText != 1 {
			t.Fatalf("lost or invented works: %+v", sc.Coverage)
		}
		if len(sc.Neighbors) != 2 || len(sc.Edges) != 0 {
			t.Fatalf("neighbors=%d, visual topic edges=%d", len(sc.Neighbors), len(sc.Edges))
		}
		for _, n := range sc.Neighbors {
			if n.WorkID == "c" || n.To == "c" || n.WorkID == "d" || n.To == "d" || len(n.Shared) != 2 {
				t.Fatalf("false neighbor %+v", n)
			}
		}
		for _, topic := range sc.Topics {
			if topic.Kind == "topic" {
				if seenTopics[topic.ID] {
					t.Fatal("topic identity merged across sources")
				}
				seenTopics[topic.ID] = true
				if topic.Label != "海水造景 · 灯光" {
					t.Fatalf("topic label not from local shared text: %q", topic.Label)
				}
			}
		}
		for _, m := range sc.TopicMembership {
			for _, e := range m.Evidence {
				title := ""
				for _, w := range sc.Works {
					if w.ID == m.WorkID {
						title = w.Title
					}
				}
				if !ClueMatch(Clean(title), e.Excerpt) {
					t.Fatal("evidence absent from real title")
				}
			}
		}
	}
	before, _ := json.Marshal(s.Scopes)
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	other, err := Analyze(context.Background(), Scope{}, rows)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(other.Scopes)
	if string(before) != string(after) {
		t.Fatal("reordering input changed sparse snapshot")
	}
	if s.AnalysisVersion == "native-literal-baseline-v2" {
		t.Fatal("stale fixed-topic caches not invalidated")
	}
}

func TestWordPhrasesDoNotCrossWhitespaceOrPunctuation(t *testing.T) {
	works := []Work{}
	for i := 0; i < 4; i++ {
		works = append(works, Work{Title: "alpha beta, gamma delta"})
	}
	for i := 0; i < 36; i++ {
		works = append(works, Work{Title: fmt.Sprintf("unique%d", i)})
	}
	docs, err := wordDocuments(context.Background(), works)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if len(d.phrases) != 0 {
			t.Fatal("learned phrase across a real boundary")
		}
	}
}

func TestValidateRejectsInvalidSparseNeighbors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"missing work", func(s *Snapshot) { s.Scopes[0].Neighbors[0].To = "missing" }},
		{"self neighbor", func(s *Snapshot) { n := &s.Scopes[0].Neighbors[0]; n.To = n.WorkID }},
		{"invalid score", func(s *Snapshot) { s.Scopes[0].Neighbors[0].Score = math.NaN() }},
		{"duplicate provenance", func(s *Snapshot) { s.Scopes[0].Neighbors[0].Shared = []string{"摄影", "摄影"} }},
		{"contained evidence", func(s *Snapshot) { s.Scopes[0].Neighbors[0].Shared = []string{"海水", "海水造景"} }},
		{"duplicate pair", func(s *Snapshot) { sc := &s.Scopes[0]; sc.Neighbors = append(sc.Neighbors, sc.Neighbors[0]) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := makeSnapshot(t)
			tt.mutate(s)
			if Validate(s) == nil {
				t.Fatal("invalid sparse relation accepted")
			}
		})
	}
}

func TestHashtagAndPlainCompoundUseSameEvidence(t *testing.T) {
	works := []Work{{Title: "#海水造景 #灯光"}, {Title: "海水造景 灯光"}}
	docs, err := wordDocuments(context.Background(), works)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(canonicalWords(docs[0]), canonicalWords(docs[1])) {
		t.Fatal("hashtag provenance changed canonical content")
	}
	g, err := sparseWordGraph(context.Background(), docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(g[0]) != 1 || math.Abs(g[0][0].score-1) > 1e-12 {
		t.Fatal("same caption content stopped matching without hashtag")
	}
}
