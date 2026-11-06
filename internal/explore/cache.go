package explore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Cache is separate from the collection database, including its atomic-write temp files.
type Cache struct{ Dir string }
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
		return err
	}
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		return err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return err
	}
	b, err := json.Marshal(cacheEnvelope{SchemaVersion: SchemaVersion, Scope: s.Scope, InputRevision: s.InputRevision, RuleHash: s.RuleHash, Checksum: checksum(body), Snapshot: body})
	if err != nil {
		return err
	}
	if len(b) > 64<<20 {
		return errors.New("SNAPSHOT_TOO_LARGE")
	}
	f, err := os.CreateTemp(c.Dir, ".snapshot-*.part")
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
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, c.path(s.Scope, s.InputRevision, s.RuleHash)); err != nil {
		return err
	}
	dir, err := os.Open(c.Dir)
	if err == nil {
		err = dir.Sync()
		dir.Close()
	}
	return err
}
func Validate(s *Snapshot) error {
	if s.SchemaVersion != SchemaVersion || s.RuleHash == "" || s.InputRevision == "" || s.SnapshotKind != "baseline" {
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
		if !sc.TimeCapabilities.Views["current"] || sc.TimeCapabilities.Views["source_time"] || sc.TimeCapabilities.Views["observed_time"] {
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
		for _, w := range sc.Works {
			if works[w.ID] || w.Source != sc.Source || w.ID == "" || w.MonthKeys.SourceTime != nil || w.MonthKeys.ObservedTime != nil {
				return errors.New("SNAPSHOT_WORK_INVALID")
			}
			if w.Cover != "" && !strings.HasPrefix(w.Cover, "hoard-cover://localhost/w/") {
				return errors.New("SNAPSHOT_ASSET_INVALID")
			}
			if w.Author.AvatarURL != "" && !strings.HasPrefix(w.Author.AvatarURL, "hoard-cover://localhost/a/") {
				return errors.New("SNAPSHOT_ASSET_INVALID")
			}
			works[w.ID] = true
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
