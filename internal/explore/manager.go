package explore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
)

type Reader func(context.Context, Scope) ([]store.ExploreRecord, error)
type TaskView struct {
	ID            string `json:"id"`
	State         string `json:"state"`
	Scope         Scope  `json:"scope"`
	InputRevision string `json:"inputRevision"`
	SnapshotID    string `json:"snapshotId"`
	CacheHit      bool   `json:"cacheHit"`
	Error         string `json:"error"`
}
type task struct {
	view   TaskView
	client string
	ctx    context.Context
	cancel context.CancelFunc
	result *Snapshot
}
type computation struct {
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	refs     int
	finished bool
	snapshot *Snapshot
	cacheHit bool
	err      error
}
type Manager struct {
	closed  bool
	wg      sync.WaitGroup
	mu      sync.Mutex
	reader  Reader
	cache   Cache
	tasks   map[string]*task
	latest  map[string]string
	work    map[string]*computation
	analyze func(context.Context, Scope, []store.ExploreRecord) (*Snapshot, error)
	after   func(*Snapshot)
}

func NewManager(reader Reader, cache Cache, after func(*Snapshot)) *Manager {
	return &Manager{reader: reader, cache: cache, tasks: map[string]*task{}, latest: map[string]string{}, work: map[string]*computation{}, analyze: Analyze, after: after}
}
func (m *Manager) Start(scope Scope, client string) (TaskView, error) {
	if client == "" || len(client) > 128 || scope.Source != "" && scope.Source != "douyin" && scope.Source != "bilibili" {
		return TaskView{}, errors.New("INVALID_REQUEST")
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return TaskView{}, errors.New("TASK_START_FAILED")
	}
	id := hex.EncodeToString(nonce)
	ctx, cancel := context.WithCancel(context.Background())
	t := &task{view: TaskView{ID: id, State: "reading", Scope: scope}, client: client, ctx: ctx, cancel: cancel}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		return TaskView{}, errors.New("SERVICE_CLOSED")
	}
	if old := m.tasks[m.latest[client]]; old != nil {
		old.cancel()
		old.view.State = "superseded"
		old.result = nil
	}
	m.tasks[id] = t
	m.latest[client] = id
	// Retain a bounded amount of terminal diagnostic state, never delete active tasks.
	if len(m.tasks) > 128 {
		for key, v := range m.tasks {
			if key != id && (v.view.State == "cancelled" || v.view.State == "superseded" || v.view.State == "failed" || v.view.State == "complete") {
				delete(m.tasks, key)
				if m.latest[v.client] == key {
					delete(m.latest, v.client)
				}
				if len(m.tasks) <= 96 {
					break
				}
			}
		}
	}
	view := t.view
	m.wg.Add(1)
	m.mu.Unlock()
	go m.run(t)
	return view, nil
}
func (m *Manager) update(t *task, state string) {
	m.mu.Lock()
	if t.ctx.Err() == nil && m.latest[t.client] == t.view.ID {
		t.view.State = state
	}
	m.mu.Unlock()
}
func (m *Manager) run(t *task) {
	defer m.wg.Done()
	LogDiagnostic("read-start", t.view.Scope, nil)
	rows, err := m.reader(t.ctx, t.view.Scope)
	if err != nil {
		LogDiagnostic("read-failed", t.view.Scope, err)
		m.fail(t, err)
		return
	}
	if t.ctx.Err() != nil {
		m.fail(t, t.ctx.Err())
		return
	}
	counts := map[string]int{"douyin": 0, "bilibili": 0}
	for _, r := range rows {
		if _, ok := counts[r.Source]; ok {
			counts[r.Source]++
		}
	}
	log.Printf("[hoard:exploration] stage=read-ok account_scoped=%t douyin_items=%d bilibili_items=%d", t.view.Scope.AccountID != "", counts["douyin"], counts["bilibili"])
	input := Revision(t.view.Scope, rows)
	key := cacheKey(t.view.Scope, input, RuleHash)
	m.mu.Lock()
	if t.ctx.Err() != nil {
		m.mu.Unlock()
		m.fail(t, t.ctx.Err())
		return
	}
	t.view.InputRevision = input
	t.view.State = "analyzing"
	shared := m.work[key]
	if shared == nil || shared.ctx.Err() != nil {
		ctx, cancel := context.WithCancel(context.Background())
		shared = &computation{ctx: ctx, cancel: cancel, done: make(chan struct{})}
		m.work[key] = shared
		m.wg.Add(1)
		go m.compute(key, shared, t.view.Scope, rows, input)
	}
	shared.refs++
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		shared.refs--
		if shared.refs == 0 {
			if !shared.finished {
				shared.cancel()
			}
			if m.work[key] == shared {
				delete(m.work, key)
			}
		}
		m.mu.Unlock()
	}()
	select {
	case <-t.ctx.Done():
		m.fail(t, t.ctx.Err())
		return
	case <-shared.done:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if t.ctx.Err() != nil || m.latest[t.client] != t.view.ID {
		LogDiagnostic("late-drop", t.view.Scope, t.ctx.Err())
		return
	}
	if shared.err != nil {
		t.view.State = "failed"
		t.view.Error = safeError(shared.err)
		return
	}
	t.result = shared.snapshot
	t.view.State = "complete"
	t.view.CacheHit = shared.cacheHit
	t.view.SnapshotID = shared.snapshot.SnapshotID
}
func (m *Manager) compute(key string, c *computation, scope Scope, rows []store.ExploreRecord, input string) {
	defer m.wg.Done()
	s, err := m.cache.Load(scope, input, RuleHash)
	hit := err == nil
	if !hit {
		LogDiagnostic("cache-miss", scope, nil)
		s, err = m.analyze(c.ctx, scope, rows)
		if err != nil {
			LogDiagnostic("analyze-failed", scope, err)
		}
		if err == nil && c.ctx.Err() == nil {
			err = m.cache.Save(s)
			if err != nil {
				LogDiagnostic("cache-save-failed", scope, err)
			}
		}
	}
	if err == nil && c.ctx.Err() != nil {
		err = c.ctx.Err()
	}
	if err == nil {
		log.Printf("[hoard:exploration] stage=complete cache_hit=%t sources=%d", hit, len(s.Scopes))
	}
	if err == nil && m.after != nil {
		m.after(s)
	}
	m.mu.Lock()
	c.snapshot = s
	c.cacheHit = hit
	c.err = err
	c.finished = true
	close(c.done)
	m.mu.Unlock()
}
func safeError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "CANCELLED"
	}
	if err.Error() == "INVALID_SCOPE" || err.Error() == "SCOPE_MISMATCH" || err.Error() == "SNAPSHOT_TOO_LARGE" {
		return err.Error()
	}
	return "EXPLORATION_FAILED"
}
func (m *Manager) fail(t *task, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.latest[t.client] != t.view.ID {
		return
	}
	if errors.Is(err, context.Canceled) || t.ctx.Err() != nil {
		t.view.State = "cancelled"
		t.view.Error = "CANCELLED"
	} else {
		t.view.State = "failed"
		t.view.Error = safeError(err)
	}
}
func (m *Manager) Status(id string) (TaskView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil {
		return TaskView{}, errors.New("TASK_NOT_FOUND")
	}
	return t.view, nil
}
func (m *Manager) Result(id string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil {
		return nil, errors.New("TASK_NOT_FOUND")
	}
	if m.latest[t.client] != id || t.view.State == "superseded" {
		return nil, errors.New("TASK_SUPERSEDED")
	}
	if t.view.State != "complete" || t.result == nil {
		return nil, errors.New("TASK_NOT_COMPLETE")
	}
	return t.result, nil
}
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.tasks[id]
	if t == nil {
		return nil
	}
	t.cancel()
	t.result = nil
	if t.view.State != "superseded" {
		t.view.State = "cancelled"
		t.view.Error = "CANCELLED"
	}
	return nil
}

// Wait is for native tests and offline local audits; it is not bound to mygo.
func (m *Manager) Wait(ctx context.Context, id string) (*Snapshot, error) {
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		v, err := m.Status(id)
		if err != nil {
			return nil, err
		}
		if v.State == "complete" {
			return m.Result(id)
		}
		if v.State == "failed" || v.State == "cancelled" || v.State == "superseded" {
			return nil, errors.New(v.State)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	for _, t := range m.tasks {
		t.cancel()
	}
	for _, c := range m.work {
		c.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
}
