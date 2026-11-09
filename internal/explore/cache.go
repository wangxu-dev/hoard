package explore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
)

// Cache is separate from the collection database, including its atomic-write temp files.
type Cache struct {
	Dir    string
	failIO func(stage string) error
}
type cacheEnvelope struct {
	SchemaVersion int             `json:"schemaVersion"`
	Scope         Scope           `json:"scope"`
	InputRevision string          `json:"inputRevision"`
	RuleHash      string          `json:"ruleHash"`
	Checksum      string          `json:"checksum"`
	Snapshot      json.RawMessage `json:"snapshot"`
}

func cacheKey(scope Scope, input, rule string) string {
	return hash([]any{SchemaVersion, InputVersion, scope, input, rule})
}
func (c Cache) path(scope Scope, input, rule string) string {
	return filepath.Join(c.Dir, cacheKey(scope, input, rule)+".json")
}
func checksum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func (c Cache) Load(scope Scope, input, rule string) (*Snapshot, error) {
	f, err := os.Open(c.path(scope, input, rule))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if err != nil || len(b) > 64<<20 {
		return nil, errors.New("CACHE_CORRUPT")
	}
	var e cacheEnvelope
	if json.Unmarshal(b, &e) != nil || e.SchemaVersion != SchemaVersion || e.Scope != scope || e.InputRevision != input || e.RuleHash != rule || checksum(e.Snapshot) != e.Checksum {
		return nil, errors.New("CACHE_INVALID")
	}
	var s Snapshot
	if json.Unmarshal(e.Snapshot, &s) != nil || s.Scope != scope || s.InputRevision != input || s.RuleHash != rule || s.SchemaVersion != SchemaVersion || s.AnalysisVersion != AnalysisVersion || s.InputVersion != InputVersion {
		return nil, errors.New("CACHE_INVALID")
	}
	if err = Validate(&s); err != nil {
		return nil, err
	}
	return &s, nil
}
func (c Cache) Save(s *Snapshot) error {
	if err := Validate(s); err != nil {
		scope := Scope{}
		if s != nil {
			scope = s.Scope
		}
		LogDiagnostic("cache-validate-failed", scope, err)
		return err
	}
	body, err := json.Marshal(s)
	if err != nil {
		LogDiagnostic("cache-encode-failed", s.Scope, err)
		return err
	}
	b, err := json.Marshal(cacheEnvelope{SchemaVersion: SchemaVersion, Scope: s.Scope, InputRevision: s.InputRevision, RuleHash: s.RuleHash, Checksum: checksum(body), Snapshot: body})
	if err != nil {
		return err
	}
	if len(b) > 64<<20 {
		err := errors.New("SNAPSHOT_TOO_LARGE")
		LogDiagnostic("cache-size-limit", s.Scope, err)
		return err
	}
	if err := c.io("mkdir", func() error { return os.MkdirAll(c.Dir, 0700) }); err != nil {
		return err
	}

	var f *os.File
	if err = c.io("create", func() error {
		var createErr error
		f, createErr = os.CreateTemp(c.Dir, ".snapshot-*.part")
		return createErr
	}); err != nil {
		return err
	}
	name := f.Name()
	defer func() { f.Close(); os.Remove(name) }()
	if err = c.io("chmod", func() error { return f.Chmod(0600) }); err == nil {
		err = c.io("write", func() error { _, e := f.Write(b); return e })
	}
	if err == nil {
		err = c.io("file-sync", f.Sync)
	}
	closeErr := c.io("close", f.Close)
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceCacheFile(name, c.path(s.Scope, s.InputRevision, s.RuleHash), c.io)
}

// Both are the app's native protocol, not external HTTP. Exact path/id/query
// equality also forbids arbitrary remote URLs, fragments and cache filling.
func validNativeAsset(ref, kind, id string) bool {
	suffix := url.PathEscape(kind) + "/" + url.PathEscape(id) + "?cacheOnly=1"
	return ref == "hoard-cover://localhost/"+suffix || ref == "http://hoard-cover.localhost/"+suffix
}

func Validate(s *Snapshot) error {
	if s == nil || s.SchemaVersion != SchemaVersion || s.RuleHash == "" || s.InputRevision == "" || s.SnapshotKind != "baseline" {
		return errors.New("SNAPSHOT_INVALID")
	}

	expectedSources := 2
	if s.Scope.Source != "" {
		expectedSources = 1
	}
	if len(s.Scopes) != expectedSources || len(s.Meta.Coverage) != expectedSources {
		return errors.New("SNAPSHOT_SCOPE_INVALID")
	}
	seenSources := map[string]bool{}
	for i, sc := range s.Scopes {
		if seenSources[sc.Source] {
			return errors.New("SNAPSHOT_SCOPE_INVALID")
		}
		seenSources[sc.Source] = true
		if !sc.TimeCapabilities.Views["current"] || sc.TimeCapabilities.Views["observed_time"] {
			return errors.New("SNAPSHOT_TIME_INVALID")
		}
		if sc.Coverage != s.Meta.Coverage[i] {
			return errors.New("SNAPSHOT_COUNTS_INVALID")
		}

		if sc.Source != "douyin" && sc.Source != "bilibili" || s.Scope.Source != "" && s.Scope.Source != sc.Source {
			return errors.New("SNAPSHOT_SCOPE_INVALID")
		}
		c := sc.Coverage
		if c.WorksTotal != len(sc.Works) || c.WorksInTopics+c.UncategorizedWorks+c.WorksNoText != len(sc.Works) {
			return errors.New("SNAPSHOT_COUNTS_INVALID")
		}
		works := map[string]bool{}
		members := map[string]bool{}
		hasPublication := false
		for _, w := range sc.Works {
			month := publicationMonth(w.PublishedAt)
			if (month == nil) != (w.MonthKeys.SourceTime == nil) || month != nil && *month != *w.MonthKeys.SourceTime || w.PublishedAt < 0 || w.PublishedAt >= 253402300800 {
				return errors.New("SNAPSHOT_TIME_INVALID")
			}
			hasPublication = hasPublication || month != nil
			if works[w.ID] || w.Source != sc.Source || w.ID == "" || w.MonthKeys.ObservedTime != nil {
				return errors.New("SNAPSHOT_WORK_INVALID")
			}
			if w.Cover != "" && !validNativeAsset(w.Cover, "w", w.ID) {
				return errors.New("SNAPSHOT_ASSET_INVALID")
			}
			if w.Author.AvatarURL != "" && !validNativeAsset(w.Author.AvatarURL, "a", w.ID) {
				return errors.New("SNAPSHOT_ASSET_INVALID")
			}
			works[w.ID] = true
		}
		if sc.TimeCapabilities.Views["source_time"] != hasPublication {
			return errors.New("SNAPSHOT_TIME_INVALID")
		}
		neighborKeys := map[string]bool{}
		neighborCounts := map[string]int{}
		for _, n := range sc.Neighbors {
			key := hash([]string{n.WorkID, n.To})
			if !works[n.WorkID] || !works[n.To] || n.WorkID == n.To || neighborKeys[key] || math.IsNaN(n.Score) || math.IsInf(n.Score, 0) || n.Score <= 0 || n.Score > 1 || len(n.Shared) == 0 {
				return errors.New("SNAPSHOT_NEIGHBOR_INVALID")
			}
			shared := wordDoc{base: map[string]int{}}
			for _, term := range n.Shared {
				shared.base[term] = 1
			}
			if len(canonicalWords(shared)) != len(n.Shared) {
				return errors.New("SNAPSHOT_NEIGHBOR_INVALID")
			}
			neighborKeys[key] = true
			neighborCounts[n.WorkID]++
			if neighborCounts[n.WorkID] > wordTopK {
				return errors.New("SNAPSHOT_NEIGHBOR_INVALID")
			}
		}
		for _, m := range sc.TopicMembership {
			if !works[m.WorkID] {
				return errors.New("SNAPSHOT_MEMBERSHIP_INVALID")
			}
			key := m.TopicID + ":" + m.WorkID
			if members[key] {
				return errors.New("SNAPSHOT_MEMBERSHIP_INVALID")
			}
			members[key] = true
		}
		kinds := map[string]string{}
		for _, t := range sc.Topics {
			kinds[t.ID] = t.Kind
			if t.WorkCount != len(t.WorkIDs) {
				return errors.New("SNAPSHOT_TOPIC_INVALID")
			}
			for _, id := range t.WorkIDs {
				if !members[t.ID+":"+id] {
					return errors.New("SNAPSHOT_TOPIC_INVALID")
				}
			}

		}
		categorized, unknown, noText := 0, 0, 0
		for _, w := range sc.Works {
			if w.AnchorTopicID == "" || !members[w.AnchorTopicID+":"+w.ID] {
				return errors.New("SNAPSHOT_ANCHOR_INVALID")
			}
			switch w.AnchorTopicID {
			case "t_no_text":
				noText++
			case "t_uncategorized":
				unknown++
			default:
				if kinds[w.AnchorTopicID] != "topic" {
					return errors.New("SNAPSHOT_ANCHOR_INVALID")
				}
				categorized++
			}
		}
		if categorized != c.WorksInTopics || unknown != c.UncategorizedWorks || noText != c.WorksNoText {
			return errors.New("SNAPSHOT_COUNTS_INVALID")
		}
	}
	return nil
}
