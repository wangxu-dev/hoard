package app

import (
	"errors"
	"log"
	"os"
	"sync"

	"github.com/wangxu-dev/hoard/internal/explore"
	"github.com/wangxu-dev/hoard/internal/store"
)

// Exploration owns native snapshot tasks. Its provider is resolved after app readiness.
type Exploration struct {
	mu       sync.Mutex
	provider func() (*store.Store, string)
	manager  *explore.Manager
}

func NewExploration(provider func() (*store.Store, string)) *Exploration {
	return &Exploration{provider: provider}
}
func (e *Exploration) get() (*explore.Manager, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.manager != nil {
		return e.manager, nil
	}
	db, dir := e.provider()
	if db == nil || dir == "" {
		return nil, errors.New("STORE_NOT_READY")
	}
	e.manager = explore.NewManager(db.ExploreInput, explore.Cache{Dir: dir}, func(s *explore.Snapshot) {
		if os.Getenv("HOARD_INTEREST_DEV") == "1" {
			log.Printf("[hoard:exploration] native_snapshot sources=%d schema=%d", len(s.Scopes), s.SchemaVersion)
			if dir := os.Getenv("HOARD_INTEREST_AUDIT_DIR"); dir != "" {
				if err := explore.WriteAudit(s, dir); err != nil {
					log.Print("[hoard:exploration] local audit write failed")
				}
			}
		}
	})
	return e.manager, nil
}
func (e *Exploration) Start(scope explore.Scope, clientID string) (explore.TaskView, error) {
	m, err := e.get()
	if err != nil {
		return explore.TaskView{}, err
	}
	return m.Start(scope, clientID)
}
func (e *Exploration) Status(id string) (explore.TaskView, error) {
	m, err := e.get()
	if err != nil {
		return explore.TaskView{}, err
	}
	return m.Status(id)
}
func (e *Exploration) Result(id string) (*explore.Snapshot, error) {
	m, err := e.get()
	if err != nil {
		return nil, err
	}
	return m.Result(id)
}
func (e *Exploration) Cancel(id string) error {
	m, err := e.get()
	if err != nil {
		return err
	}
	return m.Cancel(id)
}

func CloseExploration(e *Exploration) {
	e.mu.Lock()
	m := e.manager
	e.mu.Unlock()
	if m != nil {
		m.Close()
	}
}
