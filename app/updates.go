package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

const (
	// RepoURL is where Hoard's source, releases and issues live.
	RepoURL          = "https://github.com/wangxu-dev/hoard"
	latestReleaseURL = "https://api.github.com/repos/wangxu-dev/hoard/releases/latest"
	checkEvery       = 24 * time.Hour
	maxNotes         = 20000
)

// Updates asks GitHub for the latest release, at most once a day unless asked
// again by hand. It never downloads anything.
type Updates struct {
	mu     sync.Mutex
	client *http.Client
	url    string
	dir    func() (string, error)
	now    func() time.Time
}

func NewUpdates() *Updates {
	return &Updates{
		client: &http.Client{Timeout: 10 * time.Second},
		url:    latestReleaseURL,
		dir:    func() (string, error) { return mygo.App.Path(mygo.PathUserData) },
		now:    time.Now,
	}
}

// UpdateInfo compares the running version with the latest release.
type UpdateInfo struct {
	// Status is "available", "current" or "unknown" (never checked or no release found).
	Status  string `json:"status"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Name    string `json:"name"`
	Notes   string `json:"notes"`
	URL     string `json:"url"`
	// CheckedAt is when GitHub last answered, in Unix milliseconds; 0 if never.
	CheckedAt int64 `json:"checkedAt"`
	// Error is why this check could not reach GitHub: OFFLINE, RATE_LIMITED, NOT_FOUND or BAD_RESPONSE.
	Error string `json:"error"`
}

type releaseInfo struct {
	Tag  string `json:"tag"`
	Name string `json:"name"`
	Body string `json:"body"`
	URL  string `json:"url"`
}

// updateState is kept on disk between launches.
type updateState struct {
	AttemptedAt int64        `json:"attemptedAt"`
	CheckedAt   int64        `json:"checkedAt"`
	Release     *releaseInfo `json:"release,omitempty"`
}

// Check compares current with the latest release. Without force, a check made
// in the last day answers from what it found then.
func (u *Updates) Check(current string, force bool) (UpdateInfo, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	state := u.load()
	now := u.now()
	if !force && state.AttemptedAt > 0 && now.Sub(time.UnixMilli(state.AttemptedAt)) < checkEvery {
		return describe(current, state, ""), nil
	}
	state.AttemptedAt = now.UnixMilli()
	release, failure := u.fetch()
	if failure == "" {
		state.CheckedAt = now.UnixMilli()
		state.Release = release
	}
	u.save(state)
	log.Printf("[hoard:update] check force=%t result=%s", force, firstNonEmpty(failure, "ok"))
	return describe(current, state, failure), nil
}

// Open shows one of Hoard's GitHub pages in the browser; other addresses are refused.
func (u *Updates) Open(url string) error {
	if url != RepoURL && !strings.HasPrefix(url, RepoURL+"/") {
		return errors.New("URL_REFUSED")
	}
	return mygo.Shell.OpenExternal(url)
}

func (u *Updates) fetch() (*releaseInfo, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url, nil)
	if err != nil {
		return nil, "BAD_RESPONSE"
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Hoard")
	res, err := u.client.Do(req)
	if err != nil {
		return nil, "OFFLINE"
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		// No published release yet, or the repository is not public.
		return nil, "NOT_FOUND"
	case res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusForbidden:
		return nil, "RATE_LIMITED"
	case res.StatusCode != http.StatusOK:
		return nil, "BAD_RESPONSE"
	}
	var body struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil || parseVersion(body.TagName) == nil {
		return nil, "BAD_RESPONSE"
	}
	// Only Hoard's own release pages open from the app.
	page := body.HTMLURL
	if !strings.HasPrefix(page, RepoURL+"/") {
		page = RepoURL + "/releases"
	}
	notes := body.Body
	if len(notes) > maxNotes {
		notes = notes[:maxNotes]
	}
	return &releaseInfo{Tag: body.TagName, Name: body.Name, Body: notes, URL: page}, ""
}

func describe(current string, state updateState, failure string) UpdateInfo {
	info := UpdateInfo{Status: "unknown", Current: current, CheckedAt: state.CheckedAt, Error: failure}
	if state.Release == nil {
		return info
	}
	r := state.Release
	info.Latest, info.Name, info.Notes, info.URL = r.Tag, r.Name, r.Body, r.URL
	if newer(r.Tag, current) {
		info.Status = "available"
	} else {
		info.Status = "current"
		info.Notes = ""
	}
	return info
}

func (u *Updates) file() (string, error) {
	dir, err := u.dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "update-check.json"), nil
}

func (u *Updates) load() updateState {
	var state updateState
	if path, err := u.file(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &state)
		}
	}
	return state
}

func (u *Updates) save(state updateState) {
	path, err := u.file()
	if err != nil {
		return
	}
	if data, err := json.Marshal(state); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}

// version is a parsed semantic version; pre is empty for a final release.
type version struct {
	core [3]int
	pre  []string
}

func parseVersion(raw string) *version {
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "v")
	raw, _, _ = strings.Cut(raw, "+")
	core, pre, hasPre := strings.Cut(raw, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return nil
	}
	var v version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil
		}
		v.core[i] = n
	}
	if hasPre {
		if pre == "" {
			return nil
		}
		v.pre = strings.Split(pre, ".")
	}
	return &v
}

// newer reports whether release a is a later version than b. An unparsable b
// (a development build) is never told to update.
func newer(a, b string) bool {
	x, y := parseVersion(a), parseVersion(b)
	if x == nil || y == nil {
		return false
	}
	for i := range x.core {
		if x.core[i] != y.core[i] {
			return x.core[i] > y.core[i]
		}
	}
	// A final release is later than its pre-releases.
	switch {
	case len(x.pre) == 0 && len(y.pre) == 0:
		return false
	case len(x.pre) == 0:
		return true
	case len(y.pre) == 0:
		return false
	}
	for i := 0; i < len(x.pre) && i < len(y.pre); i++ {
		if x.pre[i] == y.pre[i] {
			continue
		}
		xn, xerr := strconv.Atoi(x.pre[i])
		yn, yerr := strconv.Atoi(y.pre[i])
		switch {
		case xerr == nil && yerr == nil:
			return xn > yn
		case xerr == nil:
			return false
		case yerr == nil:
			return true
		default:
			return x.pre[i] > y.pre[i]
		}
	}
	return len(x.pre) > len(y.pre)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
