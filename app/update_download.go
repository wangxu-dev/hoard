package app

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/netproxy"
)

type UpdateDownload struct {
	Status     string `json:"status"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
	Error      string `json:"error"`
	Enabled    bool   `json:"enabled"`
	Version    string `json:"version"`
}

type updateInstaller interface {
	Install(context.Context, func(int64, int64)) error
}

type updateTransfer struct {
	mu         sync.Mutex
	state      UpdateDownload
	restarting bool
}

func (u *Updates) DownloadStatus() UpdateDownload {
	u.transfer.mu.Lock()
	defer u.transfer.mu.Unlock()
	state := u.transfer.state
	state.Enabled = mygo.Updater.Enabled()
	if state.Status == "" {
		state.Status = "idle"
	}
	return state
}

func (u *Updates) Download(version string) error {
	if !mygo.Updater.Enabled() {
		return errors.New("UPDATES_DISABLED")
	}
	if v := parseVersion(version); v == nil || len(v.pre) > 0 {
		return errors.New("INVALID_VERSION")
	}
	u.transfer.mu.Lock()
	if u.transfer.state.Status == "downloading" || u.transfer.state.Status == "ready" {
		u.transfer.mu.Unlock()
		return nil
	}
	u.transfer.state = UpdateDownload{Status: "downloading", Version: version}
	u.transfer.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		// Pin both the manifest and archive to the version the user reviewed.
		mygo.Updater.SetSource(releaseUpdateSource{tag: "v" + strings.TrimPrefix(version, "v"), platform: u.platform})
		up, err := mygo.Updater.Check(ctx)
		if err != nil || up == nil || strings.TrimPrefix(up.Version, "v") != strings.TrimPrefix(version, "v") {
			u.transfer.finish("error", "DOWNLOAD_FAILED")
			return
		}
		u.transfer.install(ctx, up)
	}()
	return nil
}

func (t *updateTransfer) finish(status, failure string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.Status = status
	t.state.Error = failure
}
func (t *updateTransfer) install(ctx context.Context, up updateInstaller) {
	err := up.Install(ctx, func(done, total int64) {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.state.Downloaded = done
		t.state.Total = total
	})
	if err != nil {
		t.finish("error", "DOWNLOAD_FAILED")
		return
	}
	t.finish("ready", "")
}

func (u *Updates) Restart() error {
	u.transfer.mu.Lock()
	if u.transfer.state.Status != "ready" {
		u.transfer.mu.Unlock()
		return errors.New("UPDATE_NOT_READY")
	}
	if u.transfer.restarting {
		u.transfer.mu.Unlock()
		return nil
	}
	version := u.transfer.state.Version
	u.transfer.restarting = true
	u.transfer.mu.Unlock()
	if err := restartForUpdate(version); err != nil {
		u.transfer.mu.Lock()
		u.transfer.restarting = false
		u.transfer.mu.Unlock()
		log.Print("[hoard:update] restart helper could not start")
		return errors.New("UPDATE_RESTART_FAILED")
	}
	return nil
}

type releaseUpdateSource struct{ tag, platform string }

func (s releaseUpdateSource) Manifest(ctx context.Context, _ bool) (io.ReadCloser, error) {
	return s.Open(ctx, RepoURL+"/releases/download/"+s.tag+"/update-"+s.platform+".json")
}
func (s releaseUpdateSource) Open(ctx context.Context, raw string) (io.ReadCloser, error) {
	base := RepoURL + "/releases/download/" + s.tag + "/"
	if raw != base+"update-"+s.platform+".json" && raw != base+"hoard-"+strings.TrimPrefix(s.tag, "v")+"-"+s.platform+".tar.gz" {
		return nil, errors.New("UPDATE_URL_REFUSED")
	}
	transport := netproxy.Transport().Clone()
	transport.ResponseHeaderTimeout = 8 * time.Second
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()
	for _, endpoint := range []string{updateProxy + raw, raw} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Hoard")
		response, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		if response.StatusCode == http.StatusOK {
			return response.Body, nil
		}
		response.Body.Close()
	}
	return nil, errors.New("UPDATE_UNAVAILABLE")
}
