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
	want := (2 / (2 + idf*idf)) * (2.0 / 3)
	if math.Abs(b[0][0].score-want) > 1e-12 {
		t.Fatalf("score %.15f != canonical %.15f", b[0][0].score, want)
	}
}

func TestSparseWordsSingleCanonicalFamilyIsOneWeakEdge(t *testing.T) {
	d := wordDoc{base: map[string]int{"海水": 1, "造景": 1}, tags: []string{"海水造景"}, phrases: []string{"海水造景"}}
	g, err := sparseWordGraph(context.Background(), []wordDoc{d, d, {base: map[string]int{}}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if len(g[i]) != 1 || !reflect.DeepEqual(g[i][0].shared, []string{"海水造景"}) || math.Abs(g[i][0].score-.5) > 1e-12 {
			t.Fatalf("one family must be exactly one weak edge: %+v", g)
		}
	}
	if len(g[2]) != 0 {
		t.Fatal("empty text invented an edge")
	}
}

func TestSparseWordsKeepLowCosineSingleWordAndRankIndependentEvidence(t *testing.T) {
	docs := []wordDoc{{base: map[string]int{"汽车": 1, "底盘": 1}}, {base: map[string]int{"汽车": 1, "底盘": 1}}, {base: map[string]int{"汽车": 1}}, {base: map[string]int{"爱情": 1}}}
	for i := 0; i < 40; i++ {
		docs[2].base[fmt.Sprintf("onlyC%d", i)] = 1
	}
	g, err := sparseWordGraph(context.Background(), docs)
	if err != nil {
		t.Fatal(err)
	}
	if len(g[0]) != 2 || g[0][0].to != 1 || g[0][1].to != 2 {
		t.Fatalf("strong multi-word match must rank above weak car word: %+v", g[0])
	}
	weak := g[0][1]
	t.Logf("synthetic independent=%.6f, single-word=%.6f", g[0][0].score, weak.score)
	if weak.score <= 0 || weak.score >= .25 || !reflect.DeepEqual(weak.shared, []string{"汽车"}) {
		t.Fatalf("long-caption weak match was lost/overstated: %+v", weak)
	}
	if len(g[3]) != 0 {
		t.Fatal("car token linked unrelated love")
	}
}

func TestSparseWordsExcludeStopwordsSingleCharactersAndSubstringLove(t *testing.T) {
	works := []Work{{Title: "爱车 汽车"}, {Title: "爱情 恋爱"}, {Title: "今天 的 爱"}, {Title: "今天 的 爱"}}
	docs, err := wordDocuments(context.Background(), works)
	if err != nil {
		t.Fatal(err)
	}
	g, err := sparseWordGraph(context.Background(), docs)
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range g {
		if len(ns) != 0 {
			t.Fatalf("stopword, single-character, or substring ambiguity invented edge: %+v", g)
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

func TestNativeSparseNeighborsSkipThemeEncoding(t *testing.T) {
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
	for _, sc := range s.Scopes {
		if len(sc.Works) != 4 || sc.Coverage.WorksInTopics != 0 || sc.Coverage.UncategorizedWorks != 3 || sc.Coverage.WorksNoText != 1 {
			t.Fatalf("lost or invented works: %+v", sc.Coverage)
		}
		if len(sc.Neighbors) != 6 || len(sc.Edges) != 0 {
			t.Fatalf("neighbors=%d, visual topic edges=%d", len(sc.Neighbors), len(sc.Edges))
		}
		for _, n := range sc.Neighbors {
			if n.WorkID == "d" || n.To == "d" || len(n.Shared) < 1 {
				t.Fatalf("false neighbor %+v", n)
			}
		}
		for _, topic := range sc.Topics {
			if topic.Kind == "topic" {
				t.Fatalf("word-derived theme encoded: %+v", topic)
			}
		}
		for _, w := range sc.Works {
			if w.TextAvailability == "none" {
				if w.AnchorTopicID != "t_no_text" {
					t.Fatalf("no-text anchor %q", w.AnchorTopicID)
				}
				continue
			}
			if w.AnchorTopicID != "t_uncategorized" {
				t.Fatalf("text work anchored to theme %q", w.AnchorTopicID)
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
	if s.AnalysisVersion == "native-literal-baseline-v2" || s.AnalysisVersion == "native-sparse-words-v3-gse-1.3.1-zh-s" {
		t.Fatal("stale theme caches not invalidated")
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
		{"zero score", func(s *Snapshot) { s.Scopes[0].Neighbors[0].Score = 0 }},
		{"empty evidence", func(s *Snapshot) { s.Scopes[0].Neighbors[0].Shared = nil }},
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
	if len(g[0]) != 1 || math.Abs(g[0][0].score-2.0/3) > 1e-12 {
		t.Fatal("same caption content stopped matching without hashtag")
	}
}

func TestValidateAcceptsSingleWeakEvidenceAndInvalidatesV4(t *testing.T) {
	s := makeSnapshot(t)
	s.Scopes[0].Neighbors[0].Shared = []string{"汽车"}
	s.Scopes[0].Neighbors[0].Score = .03
	if err := Validate(s); err != nil {
		t.Fatalf("valid one-word low-score evidence rejected: %v", err)
	}
	if AnalysisVersion == "native-sparse-neighbors-v4-gse-1.3.1-zh-s" {
		t.Fatal("old two-evidence cache version retained")
	}
}
