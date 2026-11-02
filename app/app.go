package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wangxu-dev/hoard/internal/covers"
	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/relay"
	"github.com/wangxu-dev/hoard/internal/store"
)

type AccountEngine interface {
	UseSession(string, bool) error
	Close()
	ClearSession(string) error
	Identity(context.Context) (douyin.Identity, error)
	FetchLikesContext(context.Context, string) (douyin.LikesResult, error)
	OpenDouyin() (douyin.Status, error)
	EnsureSession() (douyin.Status, error)
	ShowDouyin() error
	HideDouyin()
	Status() douyin.Status
}

type App struct {
	mu             sync.Mutex
	requestMu      sync.Mutex
	actionMu       sync.Mutex
	frontRequests  atomic.Int32
	actionRevision atomic.Uint64
	lastRequest    time.Time
	sessionCtx     context.Context
	cancelSession  context.CancelFunc
	changed        func([]Account)
	loggedIn       func()
	Engine         AccountEngine
	Media          *relay.Relay
	Covers         *covers.Cache
	Store          *store.Store
	uid            string
	nickname       string
	accountID      string
}

func (a *App) OpenDouyin() (douyin.Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accountID == "" {
		if _, err := a.addLocked("douyin"); err != nil {
			return douyin.Status{}, err
		}
	} else if a.sessionCtx == nil || a.sessionCtx.Err() != nil {
		if err := a.restoreLocked(); err != nil {
			return douyin.Status{}, err
		}
	}
	return a.Engine.OpenDouyin()
}
func (a *App) EnsureSession() (douyin.Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accountID == "" || a.sessionCtx == nil || a.sessionCtx.Err() != nil {
		if err := a.restoreLocked(); err != nil {
			return douyin.Status{}, err
		}
	}
	if a.accountID == "" {
		return douyin.Status{}, nil
	}
	return a.Engine.EnsureSession()
}
func (a *App) ShowDouyin() error     { a.mu.Lock(); defer a.mu.Unlock(); return a.Engine.ShowDouyin() }
func (a *App) HideDouyin()           { a.mu.Lock(); defer a.mu.Unlock(); a.Engine.HideDouyin() }
func (a *App) Status() douyin.Status { a.mu.Lock(); defer a.mu.Unlock(); return a.Engine.Status() }

func (a *App) CheckLogin() (douyin.LoginState, error) {
	a.mu.Lock()
	ctx := a.sessionCtx
	ready := a.Engine.Status().Ready
	a.mu.Unlock()
	if ctx == nil || !ready {
		return douyin.LoginState{Detail: "NOT_READY"}, nil
	}
	a.requestMu.Lock()
	defer a.requestMu.Unlock()
	if ctx.Err() != nil {
		return douyin.LoginState{}, errors.New("ACCOUNT_CHANGED")
	}
	identity, err := a.Engine.Identity(ctx)
	a.lastRequest = time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if ctx.Err() != nil {
		return douyin.LoginState{}, errors.New("ACCOUNT_CHANGED")
	}
	if err != nil {
		return douyin.LoginState{}, errors.New("IDENTITY_FAILED")
	}
	if identity.AuthRequired {
		identity.Detail = "AUTH_REQUIRED"
	}
	if err := a.confirmLocked(identity); err != nil {
		return douyin.LoginState{}, err
	}
	return identity.LoginState, nil
}

func (a *App) FetchLikes(cursor string) (douyin.LikesResult, error) {
	a.mu.Lock()
	ctx := a.sessionCtx
	a.mu.Unlock()
	if ctx == nil {
		return douyin.LikesResult{}, errors.New("AUTH_REQUIRED")
	}
	a.requestMu.Lock()
	defer a.requestMu.Unlock()
	if ctx.Err() != nil {
		return douyin.LikesResult{}, errors.New("ACCOUNT_CHANGED")
	}
	result, err := a.Engine.FetchLikesContext(ctx, cursor)
	a.lastRequest = time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if ctx.Err() != nil {
		return douyin.LikesResult{}, errors.New("ACCOUNT_CHANGED")
	}
	if err != nil {
		return douyin.LikesResult{}, errors.New("REQUEST_FAILED")
	}
	if result.Error == "AUTH_REQUIRED" {
		if err := a.signedOutLocked(a.accountID); err != nil {
			return douyin.LikesResult{}, err
		}
	}
	for i := range result.Items {
		result.Items[i].SourceURL = result.Items[i].PlayURL
		result.Items[i].PlayURL = a.Media.Register(result.Items[i].PlayURL)
	}
	if err := a.persist(result, cursor == "" || cursor == "0"); err != nil {
		return douyin.LikesResult{}, errors.New("STORE_FAILED")
	}
	return result, nil
}

type cachedMetadata struct {
	Author    string   `json:"author"`
	AwemeType int      `json:"aweme_type"`
	SourceURL string   `json:"source_url"`
	Images    []string `json:"images"`
	Covers    []string `json:"covers,omitempty"`
}

func (a *App) persist(result douyin.LikesResult, head bool) error {
	if a.Store == nil || a.accountID == "" || !result.OK {
		return nil
	}
	items := make([]store.Work, len(result.Items))
	for i, item := range result.Items {
		extra, err := json.Marshal(cachedMetadata{Author: item.Author, AwemeType: item.AwemeType, SourceURL: item.SourceURL, Images: item.Images})
		if err != nil {
			return err
		}
		kind := "video"
		if len(item.Images) > 0 {
			kind = "gallery"
		}
		items[i] = store.Work{RemoteID: item.ID, Kind: kind, Title: item.Desc, PublishedAt: item.CreatedAt,
			DurationMS: item.Duration, CoverURL: item.Cover, ImageCount: len(item.Images), Extra: extra}
	}
	phase := "backfill"
	if head {
		phase = "head"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return a.Store.WriteWorkPage(ctx, store.WorkPage{AccountID: a.accountID, Kind: "like", Phase: phase,
		Cursor: result.Cursor, Complete: !result.HasMore, Works: items})
}

func (a *App) CachedLikes() (douyin.LikesResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	empty := douyin.LikesResult{OK: true, Items: []douyin.LikeItem{}}
	if a.Store == nil {
		return empty, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := a.accountID
	if id == "" {
		var err error
		id, err = a.Store.Setting(ctx, "active_account")
		if errors.Is(err, sql.ErrNoRows) {
			return empty, nil
		}
		if err != nil {
			return douyin.LikesResult{}, errors.New("STORE_FAILED")
		}
	}
	account, err := a.Store.Account(ctx, id)
	if err != nil {
		return douyin.LikesResult{}, errors.New("STORE_FAILED")
	}
	a.accountID, a.uid, a.nickname = id, account.RemoteID, account.Nickname
	state, err := a.Store.SyncState(ctx, id, "like")
	if errors.Is(err, sql.ErrNoRows) {
		return empty, nil
	}
	if err != nil {
		return douyin.LikesResult{}, errors.New("STORE_FAILED")
	}
	result := empty
	result.Cursor = state.BackfillCursor
	if state.Phase == "head" || result.Cursor == "" {
		result.Cursor = state.HeadCursor
	}
	result.HasMore = state.Phase != "done"
	var after *store.WorkCursor
	for {
		works, next, err := a.Store.ListWorks(ctx, id, "like", after, 100)
		if err != nil {
			return douyin.LikesResult{}, errors.New("STORE_FAILED")
		}
		for _, work := range works {
			var meta cachedMetadata
			if err := json.Unmarshal(work.Extra, &meta); err != nil {
				return douyin.LikesResult{}, errors.New("STORE_FAILED")
			}
			if meta.Images == nil {
				meta.Images = []string{}
			}
			result.Items = append(result.Items, douyin.LikeItem{ID: work.RemoteID, Desc: work.Title, Author: meta.Author,
				Cover: work.CoverURL, Duration: work.DurationMS, CreatedAt: work.PublishedAt, AwemeType: meta.AwemeType,
				SourceURL: meta.SourceURL, PlayURL: a.Media.Register(meta.SourceURL), Images: meta.Images})
		}
		if next == nil {
			break
		}
		after = next
	}
	sort.SliceStable(result.Items, func(i, j int) bool { return result.Items[i].CreatedAt > result.Items[j].CreatedAt })
	return result, nil
}

var languages = map[string]bool{"zh-CN": true, "en": true}

// Volume returns the saved player volume.
func (a *App) Volume() float64 {
	if a.Store == nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value, err := a.Store.Setting(ctx, "player_volume")
	if err != nil {
		return 1
	}
	var volume float64
	if _, err := fmt.Sscan(value, &volume); err != nil || volume < 0 || volume > 1 {
		return 1
	}
	return volume
}

// SetVolume stores the player volume between 0 and 1.
func (a *App) SetVolume(volume float64) error {
	if volume < 0 || volume > 1 {
		return errors.New("INVALID_VOLUME")
	}
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.Store.SetSetting(ctx, "player_volume", fmt.Sprintf("%.4f", volume))
}

// Muted reports whether the player was left muted.
func (a *App) Muted() bool {
	if a.Store == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value, err := a.Store.Setting(ctx, "player_muted")
	return err == nil && value == "1"
}

// SetMuted stores whether the player is muted.
func (a *App) SetMuted(muted bool) error {
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value := "0"
	if muted {
		value = "1"
	}
	return a.Store.SetSetting(ctx, "player_muted", value)
}

// PlayMode reports how the music player moves on: "list", "one" or "shuffle".
func (a *App) PlayMode() string {
	switch mode := a.setting("player_mode"); mode {
	case "one", "shuffle":
		return mode
	}
	return "list"
}

// SetPlayMode stores how the music player moves on.
func (a *App) SetPlayMode(mode string) error {
	if mode != "list" && mode != "one" && mode != "shuffle" {
		return errors.New("INVALID_PLAY_MODE")
	}
	return a.setSetting("player_mode", mode)
}

func (a *App) setting(key string) string {
	if a.Store == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	value, err := a.Store.Setting(ctx, key)
	if err != nil {
		return ""
	}
	return value
}

func (a *App) setSetting(key, value string) error {
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.Store.SetSetting(ctx, key, value)
}

// Language returns the interface language saved in the library; Chinese by default.
func (a *App) Language() string {
	if a.Store == nil {
		return "zh-CN"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lang, err := a.Store.Setting(ctx, "language")
	if err != nil || !languages[lang] {
		return "zh-CN"
	}
	return lang
}

// SetLanguage saves the interface language chosen in settings.
func (a *App) SetLanguage(lang string) error {
	if !languages[lang] {
		return errors.New("INVALID_LANGUAGE")
	}
	if a.Store == nil {
		return errors.New("STORE_FAILED")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Store.SetSetting(ctx, "language", lang); err != nil {
		return errors.New("STORE_FAILED")
	}
	return nil
}
