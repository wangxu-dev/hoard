//go:build !windows

package explore

import (
	"os"
	"path/filepath"
)

func replaceCacheFile(from, to string, step func(string, func() error) error) error {
	if err := step("rename", func() error { return os.Rename(from, to) }); err != nil {
		return err
	}
	var dir *os.File
	if err := step("dir-open", func() error { var err error; dir, err = os.Open(filepath.Dir(to)); return err }); err != nil {
		return err
	}
	defer dir.Close()
	err := step("dir-sync", dir.Sync)
	closeErr := step("dir-close", dir.Close)
	if err != nil {
		return err
	}
	return closeErr
}
