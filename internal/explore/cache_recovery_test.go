package explore

import (
	"context"
	"errors"
	"github.com/wangxu-dev/hoard/internal/store"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestExploreWindowsNativeAssetValidation(t *testing.T) {
	s := makeSnapshot(t)
	w := &s.Scopes[0].Works[0]
	for _, base := range []string{"hoard-cover://localhost/", "http://hoard-cover.localhost/"} {
		w.Cover = base + "w/1?cacheOnly=1"
		w.Author.AvatarURL = base + "a/1?cacheOnly=1"
		if err := Validate(s); err != nil {
			t.Fatal("valid native protocol rejected", err)
		}
	}
	for _, bad := range []string{"http://hoard-cover.localhost.evil/w/1?cacheOnly=1", "http://hoard-cover.localhost/w/other?cacheOnly=1", "http://hoard-cover.localhost/w/1?cacheOnly=0", "http://hoard-cover.localhost/w/1?cacheOnly=1#x", "https://fixture.invalid/cover"} {
		w.Cover = bad
		if err := Validate(s); err == nil {
			t.Fatal("unsafe asset accepted")
		}
	}
}
func TestExploreCacheIOFailureKeepsValidatedSnapshotAndRetries(t *testing.T) {
	stages := []string{"mkdir", "create", "chmod", "write", "file-sync", "close", "rename"}
	if runtime.GOOS != "windows" {
		stages = append(stages, "dir-open", "dir-sync", "dir-close")
	}
	for _, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			var fail atomic.Bool
			fail.Store(true)
			var attempts atomic.Int32
			cache := Cache{Dir: t.TempDir(), failIO: func(s string) error {
				if s == stage {
					attempts.Add(1)
					if fail.Load() {
						return &os.PathError{Op: "write", Path: "private-fixture-path", Err: syscall.EACCES}
					}
				}
				return nil
			}}
			m := NewManager(func(context.Context, Scope) ([]store.ExploreRecord, error) { return fixtureRows(), nil }, cache, nil)
			defer m.Close()
			first, _ := m.Start(Scope{}, "view")
			snap := waitTask(t, m, first)
			if err := Validate(snap); err != nil {
				t.Fatal(err)
			}
			v, _ := m.Status(first.ID)
			if v.State != "complete" || v.CacheWarning == "" || v.CacheHit {
				t.Fatal("I/O failure hid valid data or warning")
			}
			fail.Store(false)
			second, _ := m.Start(Scope{}, "view")
			waitTask(t, m, second)
			v, _ = m.Status(second.ID)
			if v.CacheWarning != "" || attempts.Load() < 2 {
				t.Fatal("persistence was not retried")
			}
			third, _ := m.Start(Scope{}, "view")
			waitTask(t, m, third)
			v, _ = m.Status(third.ID)
			if !v.CacheHit {
				t.Fatal("recovered cache not used")
			}
			parts, _ := filepath.Glob(filepath.Join(cache.Dir, "*.part"))
			if len(parts) != 0 {
				t.Fatal("temporary file leak")
			}
		})
	}
}
func TestExploreCacheFallbackNeverMasksCancellationOrAnalysisOrValidation(t *testing.T) {
	for _, mode := range []string{"cancel", "analysis", "invalid", "encoding"} {
		t.Run(mode, func(t *testing.T) {
			var reached atomic.Bool
			c := Cache{Dir: t.TempDir(), failIO: func(string) error { reached.Store(true); return syscall.EACCES }}
			m := NewManager(func(context.Context, Scope) ([]store.ExploreRecord, error) { return fixtureRows(), nil }, c, nil)
			defer m.Close()
			m.analyze = func(ctx context.Context, scope Scope, rows []store.ExploreRecord) (*Snapshot, error) {
				if mode == "cancel" {
					return nil, context.Canceled
				}
				if mode == "analysis" {
					return nil, errors.New("private-analysis-error")
				}
				s, err := Analyze(ctx, scope, rows)
				if mode == "invalid" {
					s.Scopes[0].Works[0].AnchorTopicID = "invalid"
				}
				if mode == "encoding" {
					s.Scopes[0].Edges = append(s.Scopes[0].Edges, Edge{Jaccard: math.NaN()})
				}
				return s, err
			}
			task, _ := m.Start(Scope{}, "view")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if _, err := m.Wait(ctx, task.ID); err == nil {
				t.Fatal("invalid result exposed")
			}
			v, _ := m.Status(task.ID)
			if mode == "cancel" && v.State != "cancelled" {
				t.Fatal("cancellation changed into failure")
			}
			if mode != "cancel" && v.State != "failed" {
				t.Fatal("failure was hidden")
			}
			if reached.Load() {
				t.Fatal("I/O attempted before validation/encoding")
			}
		})
	}
	// An actual cancellation during an I/O warning must still suppress the result.
	gate, release := make(chan struct{}), make(chan struct{})
	m := NewManager(func(context.Context, Scope) ([]store.ExploreRecord, error) { return fixtureRows(), nil }, Cache{Dir: t.TempDir(), failIO: func(stage string) error {
		if stage == "rename" {
			close(gate)
			<-release
			return syscall.EACCES
		}
		return nil
	}}, nil)
	defer m.Close()
	task, _ := m.Start(Scope{}, "view")
	<-gate
	m.Cancel(task.ID)
	close(release)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := m.Wait(ctx, task.ID); err == nil {
		t.Fatal("cancelled snapshot became available")
	}
}
func TestExploreSnapshotSizeLimitBeforeRecoverableIO(t *testing.T) {
	s := makeSnapshot(t)
	s.Scopes[0].Works[0].Title = strings.Repeat("x", 64<<20)
	reached := false
	c := Cache{Dir: t.TempDir(), failIO: func(string) error { reached = true; return syscall.EACCES }}
	if err := c.Save(s); err == nil || err.Error() != "SNAPSHOT_TOO_LARGE" {
		t.Fatal("size limit lost")
	}
	if reached {
		t.Fatal("cache I/O failure bypassed size cap")
	}
}
