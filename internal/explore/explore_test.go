package explore

import (
	"context"
	"encoding/json"
	"github.com/wangxu-dev/hoard/internal/store"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureRows() []store.ExploreRecord {
	return []store.ExploreRecord{{Source: "douyin", ID: "1", Title: "摄影旅行", Kind: "video", AuthorID: "real-author", Author: "Name", CoverURL: "https://fixture.invalid/cover", AvatarURL: "https://fixture.invalid/avatar"}, {Source: "douyin", ID: "2", Title: "摄影旅行", Kind: "gallery"}, {Source: "douyin", ID: "3", Title: "摄影旅行", Kind: "video"}, {Source: "douyin", ID: "4", Title: "无匹配的短语", Kind: "video"}, {Source: "douyin", ID: "5", Title: "", Kind: "video"}}
}
func makeSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	s, err := Analyze(context.Background(), Scope{}, fixtureRows())
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestExploreNativeRulesCountsAndCoMembership(t *testing.T) {
	s := makeSnapshot(t)
	if err := Validate(s); err != nil {
		t.Fatal(err)
	}
	for _, sc := range s.Scopes {
		for _, w := range sc.Works {
			if w.AnchorTopicID == "" {
				t.Fatal("native anchor missing")
			}
		}
	}
	c := s.Scopes[0].Coverage
	if c.WorksTotal != 5 || c.WorksInTopics != 3 || c.UncategorizedWorks != 1 || c.WorksNoText != 1 {
		t.Fatalf("counts %+v", c)
	}
	if !ClueMatch("玩 cs 游戏", "cs") || ClueMatch("classic music", "cs") || ClueMatch("瞬间", "瞬") {
		t.Fatal("boundary/CJK rule changed")
	}
	if Clean(" @someone 摄影  HTTPS://fixture.invalid/a ") != "摄影" {
		t.Fatal("cleaning changed")
	}
	for _, sc := range s.Scopes {
		for _, edge := range sc.Edges {
			a, b := map[string]bool{}, map[string]bool{}
			for _, m := range sc.TopicMembership {
				if m.TopicID == edge.Source {
					a[m.WorkID] = true
				}
				if m.TopicID == edge.Target {
					b[m.WorkID] = true
				}
			}
			co := 0
			for id := range a {
				if b[id] {
					co++
				}
			}
			if co != edge.CoCount || edge.Jaccard != float64(co)/float64(len(a)+len(b)-co) {
				t.Fatal("coCount/Jaccard inconsistent")
			}
		}
	}
	rows := fixtureRows()
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if Revision(Scope{}, rows) != s.InputRevision {
		t.Fatal("input order changed version")
	}
	other, _ := Analyze(context.Background(), Scope{AccountID: "other"}, rows)
	if other.SnapshotID == s.SnapshotID {
		t.Fatal("scope not included in version")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Analyze(ctx, Scope{}, rows); err == nil {
		t.Fatal("analysis cancellation ignored")
	}
}
func TestExploreCacheHitInvalidationAndCorruption(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	s := makeSnapshot(t)
	if err := c.Save(s); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Load(s.Scope, s.InputRevision, RuleHash); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct {
		s    Scope
		i, r string
	}{{Scope{AccountID: "other"}, s.InputRevision, RuleHash}, {s.Scope, "new-input", RuleHash}, {s.Scope, s.InputRevision, "new-rule"}} {
		if _, err := c.Load(pair.s, pair.i, pair.r); err == nil {
			t.Fatal("cache crossed input/rule/scope boundary")
		}
	}
	path := c.path(s.Scope, s.InputRevision, RuleHash)
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Load(s.Scope, s.InputRevision, RuleHash); err == nil {
		t.Fatal("corrupt cache accepted")
	}
	if err := c.Save(s); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatal("cache permissions")
	}
	files, _ := filepath.Glob(filepath.Join(c.Dir, "*.part"))
	if len(files) != 0 {
		t.Fatal("atomic temp leaked")
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Save(s); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := c.Load(s.Scope, s.InputRevision, RuleHash); err != nil {
		t.Fatal("concurrent atomic writes corrupt")
	}
}
func waitTask(t *testing.T, m *Manager, v TaskView) *Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := m.Wait(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestExploreTasksDeduplicateAndCacheFallback(t *testing.T) {
	m := NewManager(func(context.Context, Scope) ([]store.ExploreRecord, error) { return fixtureRows(), nil }, Cache{Dir: t.TempDir()}, nil)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	m.analyze = func(ctx context.Context, s Scope, r []store.ExploreRecord) (*Snapshot, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
		}
		return Analyze(ctx, s, r)
	}
	a, _ := m.Start(Scope{}, "client-a")
	<-entered
	b, _ := m.Start(Scope{}, "client-b")
	deadline := time.Now().Add(time.Second)
	for {
		m.mu.Lock()
		refs := 0
		for _, c := range m.work {
			refs = c.refs
		}
		m.mu.Unlock()
		if refs == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("tasks did not share analysis")
		}
		time.Sleep(time.Millisecond)
	}
	if err := m.Cancel(a.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	s := waitTask(t, m, b)
	if calls.Load() != 1 {
		t.Fatal("duplicate analysis")
	}
	if _, err := m.Result(a.ID); err == nil {
		t.Fatal("cancelled late result exposed")
	}
	c, _ := m.Start(Scope{}, "client-c")
	waitTask(t, m, c)
	status, _ := m.Status(c.ID)
	if !status.CacheHit || calls.Load() != 1 {
		t.Fatal("persistent hit not reused")
	}
	if err := os.WriteFile(m.cache.path(s.Scope, s.InputRevision, s.RuleHash), []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	d, _ := m.Start(Scope{}, "client-d")
	waitTask(t, m, d)
	if calls.Load() != 2 {
		t.Fatal("corruption did not recompute")
	}
}
func TestExploreNewRequestSupersedesOldAccountAndInput(t *testing.T) {
	gate := make(chan struct{})
	m := NewManager(func(ctx context.Context, s Scope) ([]store.ExploreRecord, error) {
		if s.AccountID == "old" {
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return fixtureRows(), nil
	}, Cache{Dir: t.TempDir()}, nil)
	old, _ := m.Start(Scope{AccountID: "old"}, "view")
	next, _ := m.Start(Scope{AccountID: "new"}, "view")
	waitTask(t, m, next)
	close(gate)
	if _, err := m.Result(old.ID); err == nil {
		t.Fatal("old account result overwrote new")
	}
	if v, _ := m.Status(old.ID); v.State != "superseded" {
		t.Fatal(v.State)
	}
}
func TestExploreAuditIsPrivateAndDoesNotInventSemanticVerdict(t *testing.T) {
	dir := t.TempDir()
	s := makeSnapshot(t)
	if err := WriteAudit(s, dir); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Fatal("audit missing")
	}
	b, _ := os.ReadFile(files[0])
	var r struct {
		SemanticReviewed int `json:"semanticReviewed"`
		Samples          []struct {
			SemanticVerdict string `json:"semanticVerdict"`
		} `json:"samples"`
	}
	if json.Unmarshal(b, &r) != nil || r.SemanticReviewed != 0 || len(r.Samples) == 0 {
		t.Fatal("invalid audit")
	}
	for _, sample := range r.Samples {
		if sample.SemanticVerdict != "PENDING_LOCAL_HUMAN_REVIEW" {
			t.Fatal("invented semantic verdict")
		}
	}
}

func TestExploreInputChangeInvalidatesNativeCache(t *testing.T) {
	var mu sync.Mutex
	rows := fixtureRows()
	m := NewManager(func(context.Context, Scope) ([]store.ExploreRecord, error) {
		mu.Lock()
		defer mu.Unlock()
		return append([]store.ExploreRecord{}, rows...), nil
	}, Cache{Dir: t.TempDir()}, nil)
	defer m.Close()
	a, _ := m.Start(Scope{}, "view")
	first := waitTask(t, m, a)
	mu.Lock()
	rows[0].Title = "不同标题"
	mu.Unlock()
	b, _ := m.Start(Scope{}, "view")
	next := waitTask(t, m, b)
	v, _ := m.Status(b.ID)
	if v.CacheHit || first.InputRevision == next.InputRevision {
		t.Fatal("new input reused old cache")
	}
	if _, err := m.Result(a.ID); err == nil {
		t.Fatal("old version exposed after new request")
	}
}
func TestExploreServiceCloseCancelsAndJoinsTasks(t *testing.T) {
	entered := make(chan struct{})
	m := NewManager(func(ctx context.Context, _ Scope) ([]store.ExploreRecord, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}, Cache{Dir: t.TempDir()}, nil)
	v, _ := m.Start(Scope{}, "view")
	<-entered
	m.Close()
	if _, err := m.Result(v.ID); err == nil {
		t.Fatal("closed task exposed result")
	}
	if _, err := m.Start(Scope{}, "next"); err == nil {
		t.Fatal("closed service accepted task")
	}
}

func TestExploreAuditPreservesExistingLocalReview(t *testing.T) {
	dir := t.TempDir()
	s := makeSnapshot(t)
	if err := WriteAudit(s, dir); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	note := []byte("local human review retained")
	if err := os.WriteFile(files[0], note, 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteAudit(s, dir); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(files[0])
	if string(b) != string(note) {
		t.Fatal("cache hit overwrote human audit")
	}
}

func TestExploreCacheRejectsWrongEnvelopeAndInventedHistory(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	s := makeSnapshot(t)
	if err := c.Save(s); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(c.path(s.Scope, s.InputRevision, RuleHash))
	if err != nil {
		t.Fatal(err)
	}
	path := c.path(s.Scope, "changed-input", RuleHash)
	if err = os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Load(s.Scope, "changed-input", RuleHash); err == nil {
		t.Fatal("misfiled old revision accepted")
	}
	path = c.path(s.Scope, s.InputRevision, "changed-rule")
	os.WriteFile(path, b, 0600)
	if _, err = c.Load(s.Scope, s.InputRevision, "changed-rule"); err == nil {
		t.Fatal("misfiled old rule accepted")
	}
	s.Scopes[0].TimeCapabilities.Views["source_time"] = true
	if err = c.Save(s); err == nil {
		t.Fatal("fabricated time capability accepted")
	}
}
