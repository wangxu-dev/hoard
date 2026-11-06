//go:build windows

package explore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExploreWindowsWriteThroughReplacement(t *testing.T) {
	d := t.TempDir()
	from, to := filepath.Join(d, "from"), filepath.Join(d, "to")
	os.WriteFile(from, []byte("new"), 0600)
	os.WriteFile(to, []byte("old"), 0600)
	if err := replaceCacheFile(from, to, func(stage string, f func() error) error {
		if strings.HasPrefix(stage, "dir-") {
			t.Fatal("unsupported directory flush")
		}
		return f()
	}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(to)
	if err != nil || string(b) != "new" {
		t.Fatal("replacement failed")
	}
}
