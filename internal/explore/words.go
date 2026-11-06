package explore

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/go-ego/gse"
)

const wordTopK = 12
const wordPostingLimit = 128
const wordMinCosine = .25

// WorkNeighbor is evidence for a work pair, not a line to draw in the sky.
type WorkNeighbor struct {
	WorkID string   `json:"workId"`
	To     string   `json:"to"`
	Score  float64  `json:"score"`
	Shared []string `json:"shared"`
}

type wordDoc struct {
	base          map[string]int
	tags, phrases []string
}
type wordNeighbor struct {
	to     int
	score  float64
	shared []string
}

var wordTags = regexp.MustCompile(`#[\p{Han}\p{L}\p{N}_+\-]{1,48}`)
var wordRun = regexp.MustCompile(`[\p{L}\p{N}_+\-]+`)
var wordStops = func() map[string]bool {
	out := map[string]bool{}
	// Function words, not interest categories. Topic labels come only from shared text.
	for _, w := range strings.Fields("的 了 是 在 和 与 或 及 也 都 就 又 把 被 让 给 很 太 更 最 这 那 这个 那个 一个 我 你 他 她 它 我们 你们 他们 自己 什么 怎么 为什么 可以 但是 因为 所以 然后 如果 还是 没有 不是 还有 真的 一起 一下 啊 呀 哦 呢 吧 嘛 哈 哈哈 的话 时候 今天 明天 现在 已经 进行 the a an and or of to in on for is are was were this that it you your i my we our with at by as be from") {
		out[w] = true
	}
	return out
}()
var wordTokenizer struct {
	once sync.Once
	mu   sync.Mutex
	seg  gse.Segmenter
	err  error
}

func cutWords(text string) ([]string, error) {
	wordTokenizer.once.Do(func() {
		wordTokenizer.seg.SkipLog = true
		wordTokenizer.seg.SkipSubSeg = true
		// Bundled general-purpose dictionary: no model/file/network download at runtime.
		wordTokenizer.err = wordTokenizer.seg.LoadDictEmbed("zh_s")
	})
	if wordTokenizer.err != nil {
		return nil, fmt.Errorf("initialize local word tokenizer: %w", wordTokenizer.err)
	}
	wordTokenizer.mu.Lock()
	defer wordTokenizer.mu.Unlock()
	return wordTokenizer.seg.Cut(text, true), nil
}

func usableWord(s string) bool {
	n := utf8.RuneCountInString(s)
	if n < 2 || n > 48 || wordStops[s] {
		return false
	}
	letter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			letter = true
		} else if !unicode.IsDigit(r) && r != '_' && r != '+' && r != '-' {
			return false
		}
	}
	return letter
}

func wordPhrase(parts []string) string {
	if len(parts) < 2 {
		return ""
	}
	for _, p := range parts {
		if !usableWord(p) {
			return ""
		}
	}
	word := strings.Join(parts, "")
	if utf8.RuneCountInString(word) > 24 {
		return ""
	}
	return word
}

func wordDocuments(ctx context.Context, works []Work) ([]wordDoc, error) {
	docs := make([]wordDoc, len(works))
	raw := make([][]string, len(works))
	baseDF, phraseDF := map[string]int{}, map[string]int{}
	phraseParts := map[string][]string{}
	for i, w := range works {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		d := wordDoc{base: map[string]int{}}
		text := Clean(w.Title)
		for _, tag := range wordTags.FindAllString(text, -1) {
			if t := strings.TrimPrefix(tag, "#"); usableWord(t) {
				d.tags = append(d.tags, t)
			}
		}
		seen := map[string]bool{}
		// Split on real boundaries before learning phrases: never join across punctuation or whitespace.
		for _, run := range wordRun.FindAllString(text, -1) {
			parts, err := cutWords(run)
			if err != nil {
				return nil, err
			}
			raw[i] = append(raw[i], parts...)
			raw[i] = append(raw[i], "")
			for _, t := range parts {
				if usableWord(t) {
					d.base[t]++
				}
			}
			for j := range parts {
				for size := 2; size <= 3 && j+size <= len(parts); size++ {
					t := wordPhrase(parts[j : j+size])
					if t != "" && !seen[t] {
						seen[t] = true
						phraseDF[t]++
						phraseParts[t] = parts[j : j+size]
					}
				}
			}
		}
		for t := range d.base {
			baseDF[t]++
		}
		docs[i] = d
	}
	accepted := map[string]bool{}
	// A literal compound learned from a hashtag must have the same representation
	// in an ordinary caption. Otherwise removing its token pieces only in the
	// hashtagged document would make identical content stop matching.
	for _, d := range docs {
		for _, t := range d.tags {
			accepted[t] = true
		}
	}
	for t, count := range phraseDF {
		if count < 4 || baseDF[t] > 0 || count > len(docs)/5 {
			continue
		}
		parts := phraseParts[t]
		den := 1.0
		for _, p := range parts {
			den *= float64(baseDF[p])
		}
		if den > 0 && math.Log2(float64(count)*math.Pow(float64(len(docs)), float64(len(parts)-1))/den) >= 3 {
			accepted[t] = true
		}
	}
	for i, parts := range raw {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		for j := range parts {
			if parts[j] == "" {
				continue
			}
			term := ""
			for k := j; k < len(parts) && parts[k] != ""; k++ {
				term += parts[k]
				if utf8.RuneCountInString(term) > 48 {
					break
				}
				if accepted[term] {
					seen[term] = true
				}
			}
		}
		for t := range seen {
			docs[i].phrases = append(docs[i].phrases, t)
		}
		sort.Strings(docs[i].phrases)
	}
	return docs, nil
}

// canonicalWords collapses provenance and containment BEFORE DF, vector norms,
// candidate selection and dot products. A tag/phrase plus its token pieces is
// one evidence family, not several votes. Binary TF also prevents repeated
// hashtag/body copies of the same word from increasing pair weight.
func canonicalWords(d wordDoc) []string {
	set := map[string]bool{}
	for t, c := range d.base {
		if c > 0 && usableWord(t) {
			set[t] = true
		}
	}
	for _, ts := range [][]string{d.tags, d.phrases} {
		for _, t := range ts {
			if usableWord(t) {
				set[t] = true
			}
		}
	}
	all := make([]string, 0, len(set))
	for t := range set {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := utf8.RuneCountInString(all[i]), utf8.RuneCountInString(all[j])
		if a != b {
			return a > b
		}
		return all[i] < all[j]
	})
	kept := []string{}
	for _, t := range all {
		contained := false
		for _, long := range kept {
			if ClueMatch(long, t) {
				contained = true
				break
			}
		}
		if !contained {
			kept = append(kept, t)
		}
	}
	sort.Strings(kept)
	return kept
}

func sparseWordGraph(ctx context.Context, docs []wordDoc) ([][]wordNeighbor, error) {
	n := len(docs)
	terms := make([][]string, n)
	vec := make([]map[string]float64, n)
	norms := make([]float64, n)
	post := map[string][]int{}
	for i, d := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		terms[i] = canonicalWords(d)
		for _, t := range terms[i] {
			post[t] = append(post[t], i)
		}
	}
	for i, ts := range terms {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		v := map[string]float64{}
		for _, t := range ts {
			w := 1 + math.Log(float64(n+1)/float64(len(post[t])+1))
			v[t] = w
			norms[i] += w * w
		}
		vec[i] = v
		norms[i] = math.Sqrt(norms[i])
	}
	out := make([][]wordNeighbor, n)
	for i, ts := range terms {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		anchors := []string{}
		for _, t := range ts {
			if len(post[t]) >= 2 && len(post[t]) <= wordPostingLimit {
				anchors = append(anchors, t)
			}
		}
		sort.Slice(anchors, func(a, b int) bool {
			if vec[i][anchors[a]] != vec[i][anchors[b]] {
				return vec[i][anchors[a]] > vec[i][anchors[b]]
			}
			return anchors[a] < anchors[b]
		})
		if len(anchors) > wordTopK {
			anchors = anchors[:wordTopK]
		}
		candidates := map[int]bool{}
		for _, t := range anchors {
			for _, j := range post[t] {
				if j != i {
					candidates[j] = true
				}
			}
		}
		for j := range candidates {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if norms[i] == 0 || norms[j] == 0 {
				continue
			}
			shared := []string{}
			dot := 0.0
			// Sorted accumulation makes results stable across map iteration and cold caches.
			for _, t := range ts {
				if w, ok := vec[j][t]; ok {
					shared = append(shared, t)
					dot += vec[i][t] * w
				}
			}
			if len(shared) < 2 {
				continue
			}
			score := math.Min(1, dot/(norms[i]*norms[j]))
			if score >= wordMinCosine {
				out[i] = append(out[i], wordNeighbor{j, score, shared})
			}
		}
		sort.Slice(out[i], func(a, b int) bool {
			if out[i][a].score != out[i][b].score {
				return out[i][a].score > out[i][b].score
			}
			return out[i][a].to < out[i][b].to
		})
		if len(out[i]) > wordTopK {
			out[i] = out[i][:wordTopK]
		}
	}
	return out, nil
}

// Neighbors are the only association. No word-derived topics or theme rings:
// works stay in flat buckets so the sky can scatter isolates and pull pairs close.
func associateWords(ctx context.Context, s *SourceSnapshot) error {
	docs, err := wordDocuments(ctx, s.Works)
	if err != nil {
		return err
	}
	graph, err := sparseWordGraph(ctx, docs)
	if err != nil {
		return err
	}
	for i, ns := range graph {
		if err := ctx.Err(); err != nil {
			return err
		}
		for _, n := range ns {
			s.Neighbors = append(s.Neighbors, WorkNeighbor{s.Works[i].ID, s.Works[n.to].ID, n.score, n.shared})
		}
	}
	for _, bucket := range []struct{ id, label, reason string }{{"t_uncategorized", "未关联", "仅保留稀疏词语邻居，不做主题分组"}, {"t_no_text", "无文案", "无文案"}} {
		ids := []string{}
		for i, w := range s.Works {
			if w.AnchorTopicID != "" {
				continue
			}
			if (bucket.id == "t_no_text") != (w.TextAvailability == "none") {
				continue
			}
			s.Works[i].AnchorTopicID = bucket.id
			ids = append(ids, w.ID)
			s.TopicMembership = append(s.TopicMembership, Membership{WorkID: w.ID, TopicID: bucket.id, Method: "fallback", Confidence: "low", Reason: bucket.reason, Evidence: []Evidence{}})
		}
		s.Topics = append(s.Topics, Topic{ID: bucket.id, Label: bucket.label, Kind: "bucket", WorkCount: len(ids), WorkIDs: ids})
		if bucket.id == "t_no_text" {
			s.Coverage.WorksNoText = len(ids)
		} else {
			s.Coverage.UncategorizedWorks = len(ids)
		}
	}
	s.Coverage.WorksInTopics = 0
	return ctx.Err()
}
