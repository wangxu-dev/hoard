package app

import (
	"context"
	"errors"
	"testing"
)

type fakeUpdateInstaller struct{ failure bool }

func (f fakeUpdateInstaller) Install(_ context.Context, progress func(int64, int64)) error {
	progress(50, 100)
	if f.failure {
		return errors.New("signature mismatch")
	}
	progress(100, 100)
	return nil
}

func TestUpdateInstallState(t *testing.T) {
	for _, failed := range []bool{false, true} {
		transfer := updateTransfer{state: UpdateDownload{Status: "downloading", Version: "v0.2.0"}}
		transfer.install(context.Background(), fakeUpdateInstaller{failed})
		if failed {
			if transfer.state.Status != "error" || transfer.state.Error != "DOWNLOAD_FAILED" {
				t.Fatal(transfer.state)
			}
		} else if transfer.state.Status != "ready" || transfer.state.Downloaded != 100 {
			t.Fatal(transfer.state)
		}
	}
}
func TestRestartRequiresVerifiedUpdate(t *testing.T) {
	for _, status := range []string{"", "idle", "downloading", "error"} {
		u := NewUpdates()
		u.transfer.state.Status = status
		if err := u.Restart(); err == nil {
			t.Fatalf("restart permitted in %q", status)
		}
	}
}
func TestUpdateSourceRefusesUnrelatedURLs(t *testing.T) {
	source := releaseUpdateSource{tag: "v0.2.0", platform: "darwin-arm64"}
	for _, raw := range []string{"https://evil.example/update.tar.gz", RepoURL + "/releases/download/v0.1.0/hoard-0.1.0-darwin-arm64.tar.gz", RepoURL + "/releases/download/v0.2.0/hoard-0.2.0-windows-amd64.tar.gz"} {
		if _, err := source.Open(context.Background(), raw); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}
