package app

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMacUpdateRestart(t *testing.T) {
	for _, tc := range []struct {
		name      string
		installed bool
		openFails bool
		want      string
	}{
		{"wait for exit and install", true, false, ""},
		{"installation failure", false, false, "installation timed out"},
		{"launch failure", true, true, "open failed status=7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bundle := filepath.Join(dir, "Hoard's Test.app")
			if err := os.MkdirAll(filepath.Join(bundle, "Contents"), 0700); err != nil {
				t.Fatal(err)
			}
			writeVersion := func(version string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleShortVersionString</key><string>`+version+`</string></dict></plist>`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeVersion("0.1.0")
			// The stub exercises the launcher, without opening an application in unit tests.
			stub := filepath.Join(dir, "open-stub")
			body := `#!/bin/sh
[ "$1" = -n ] && [ "$3" = --args ] && [ "$4" = --hoard-update-restart ] || exit 9
touch "$(dirname "$2")/open-called"
`
			if tc.openFails {
				body += "exit 7\n"
			}
			if err := os.WriteFile(stub, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			old := exec.Command("/bin/sleep", "30")
			if err := old.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = old.Process.Kill() }()
			cmd := macUpdateRestartCommand(old.Process.Pid, bundle, "v0.2.0")
			// Shorten deadlines, retaining real shell/process/PlistBuddy behavior.
			cmd.Args[2] = strings.ReplaceAll(cmd.Args[2], "remaining=480", "remaining=8")
			cmd.Args[2] = strings.ReplaceAll(cmd.Args[2], "/usr/bin/open", `"`+stub+`"`)
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond)
			if _, err := os.Stat(filepath.Join(dir, "open-called")); err == nil {
				t.Fatal("opened before the old process exited")
			}
			_ = old.Process.Kill()
			_ = old.Wait()
			if tc.installed {
				time.Sleep(50 * time.Millisecond)
				writeVersion("0.2.0")
			}
			err := cmd.Wait()
			if (err == nil) != (tc.want == "") {
				t.Fatalf("exit=%v output=%s", err, output.String())
			}
			if !strings.Contains(output.String(), tc.want) {
				t.Fatalf("missing %q in %s", tc.want, output.String())
			}
			if tc.want == "" && output.Len() != 0 {
				t.Fatalf("successful restart wrote diagnostics: %s", output.String())
			}
			if !tc.installed {
				if _, err := os.Stat(filepath.Join(dir, "open-called")); err == nil {
					t.Fatal("launched an uninstalled version")
				}
			}
		})
	}
}
