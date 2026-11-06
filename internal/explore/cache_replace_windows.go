//go:build windows

package explore

import (
	"errors"
	"golang.org/x/sys/windows"
	"time"
)

// WebView2 hosts use NTFS/Windows rename semantics: flush the file first (Save),
// then use write-through replacement. Do not FlushFileBuffers on os.Open(dir).
func replaceCacheFile(from, to string, step func(string, func() error) error) error {
	return step("rename", func() error {
		a, err := windows.UTF16PtrFromString(from)
		if err != nil {
			return err
		}
		b, err := windows.UTF16PtrFromString(to)
		if err != nil {
			return err
		}
		for attempt := 0; ; attempt++ {
			err = windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
			if err == nil || attempt >= 3 || (!errors.Is(err, windows.ERROR_ACCESS_DENIED) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION)) {
				return err
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
}
