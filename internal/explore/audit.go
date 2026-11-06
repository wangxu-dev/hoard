package explore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteAudit produces a private, reproducible review queue, not a semantic verdict.
// It is called only when an explicitly configured local dev audit directory exists.
func WriteAudit(s *Snapshot, dir string) error {
	type row struct {
		Source              string       `json:"source"`
		WorkID              string       `json:"workId"`
		Title               string       `json:"title"`
		Stratum             string       `json:"stratum"`
		Evidence            []Membership `json:"evidence"`
		MechanicalValid     bool         `json:"mechanicalValid"`
		AmbiguityCandidates []string     `json:"ambiguityCandidates"`
		SemanticVerdict     string       `json:"semanticVerdict"`
	}
	samples := []row{}
	checked, ambiguous := 0, 0
	for _, sc := range s.Scopes {
		works := map[string]Work{}
		ms := map[string][]Membership{}
		groups := map[string][]string{}
		for _, w := range sc.Works {
			works[w.ID] = w
		}
		for _, m := range sc.TopicMembership {
			ms[m.WorkID] = append(ms[m.WorkID], m)
		}
		for id, list := range ms {
			sort.Slice(list, func(i, j int) bool {
				a, b := list[i], list[j]
				if len(a.Evidence) != len(b.Evidence) {
					return len(a.Evidence) > len(b.Evidence)
				}
				longest := func(m Membership) int {
					v := 0
					for _, e := range m.Evidence {
						if len([]rune(e.Excerpt)) > v {
							v = len([]rune(e.Excerpt))
						}
					}
					return v
				}
				if longest(a) != longest(b) {
					return longest(a) > longest(b)
				}
				return a.TopicID < b.TopicID
			})
			ms[id] = list
			groups[works[id].AnchorTopicID] = append(groups[works[id].AnchorTopicID], id)
		}
		keys := []string{}
		for g := range groups {
			keys = append(keys, g)
		}
		sort.Strings(keys)
		for _, g := range keys {
			ids := groups[g]
			sort.Slice(ids, func(i, j int) bool { return hash(sc.Source+":"+ids[i]) < hash(sc.Source+":"+ids[j]) })
			limit := 3
			if g == "t_uncategorized" {
				limit = 8
			}
			if g == "t_no_text" {
				limit = 4
			}
			if len(ids) > limit {
				ids = ids[:limit]
			}
			for _, id := range ids {
				w := works[id]
				ok := true
				flags := []string{}
				for _, m := range ms[id] {
					for _, e := range m.Evidence {
						if !ClueMatch(Clean(w.Title), e.Excerpt) {
							ok = false
						}
						if len(m.Evidence) == 1 && strings.Contains("|瞬间|治愈|网络|系统|", "|"+e.Excerpt+"|") {
							flags = append(flags, e.Excerpt+"：单一宽泛词仅为歧义候选，不能证明兴趣")
						}
					}
				}
				if g == "t_no_text" && strings.TrimSpace(w.Title) != "" {
					ok = false
				}
				if len(flags) > 0 {
					ambiguous++
				}
				samples = append(samples, row{Source: sc.Source, WorkID: id, Title: w.Title, Stratum: g, Evidence: ms[id], MechanicalValid: ok, AmbiguityCandidates: flags, SemanticVerdict: "PENDING_LOCAL_HUMAN_REVIEW"})
				checked++
			}

		}
		// Deliberately sample ambiguous one-clue matches as well as random strata.
		// These are review candidates, never an invented false-positive verdict.
		seen := map[string]bool{}
		for _, sample := range samples {
			if sample.Source == sc.Source {
				seen[sample.WorkID] = true
			}
		}
		candidates := []string{}
		for id, list := range ms {
			if seen[id] {
				continue
			}
			for _, m := range list {
				if len(m.Evidence) == 1 && strings.Contains("|瞬间|治愈|网络|系统|", "|"+m.Evidence[0].Excerpt+"|") {
					candidates = append(candidates, id)
					break
				}
			}
		}
		sort.Slice(candidates, func(i, j int) bool { return hash(sc.Source+":"+candidates[i]) < hash(sc.Source+":"+candidates[j]) })
		if len(candidates) > 4 {
			candidates = candidates[:4]
		}
		for _, id := range candidates {
			w := works[id]
			flags := []string{}
			valid := true
			for _, m := range ms[id] {
				for _, e := range m.Evidence {
					if !ClueMatch(Clean(w.Title), e.Excerpt) {
						valid = false
					}
					if len(m.Evidence) == 1 && strings.Contains("|瞬间|治愈|网络|系统|", "|"+e.Excerpt+"|") {
						flags = append(flags, e.Excerpt+"：单一宽泛词，需人工判断是否误归类，不能据此认定兴趣")
					}
				}
			}
			samples = append(samples, row{Source: sc.Source, WorkID: id, Title: w.Title, Stratum: "ambiguity-review", Evidence: ms[id], MechanicalValid: valid, AmbiguityCandidates: flags, SemanticVerdict: "PENDING_LOCAL_HUMAN_REVIEW"})
			checked++
			ambiguous++
		}
	}
	report := struct {
		SnapshotID          string   `json:"snapshotId"`
		Scope               Scope    `json:"scope"`
		Criteria            []string `json:"criteria"`
		MechanicalSamples   int      `json:"mechanicalSamples"`
		AmbiguityCandidates int      `json:"ambiguityCandidates"`
		SemanticReviewed    int      `json:"semanticReviewed"`
		Samples             []row    `json:"samples"`
	}{SnapshotID: s.SnapshotID, Scope: s.Scope, Criteria: []string{"证据须满足真实标题清洗后的字面匹配及ASCII边界/CJK最少两字", "unknown/noText为未判定，不判为没有兴趣", "单个宽泛词、反语、否定、引用或跨语境均需本地人工确认；词命中不等于完整兴趣", "人工保留/误命中/不确定的判定与原因须写在此本地文件，未审不得声称语义准确"}, MechanicalSamples: checked, AmbiguityCandidates: ambiguous, Samples: samples}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	path := filepath.Join(dir, "quality-audit-"+hash(s.Scope)[:16]+"-"+s.InputRevision[:8]+"-"+s.RuleHash[:8]+"-v2.json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	f, err := os.CreateTemp(dir, ".quality-*.part")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		return err
	}
	// A completed local review is never overwritten by a cache hit or duplicate task.
	if err = os.Link(name, path); os.IsExist(err) {
		return nil
	}
	return err
}
