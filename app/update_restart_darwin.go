package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

const updateRestartFlag = "--hoard-update-restart"

// The SDK installs after exit; the launcher waits for that swap before opening.
const macUpdateRestartScript = `pid=$1 bundle=$2 version=$3
trap '' HUP
report() { printf '%s [hoard:update-restart] %s\n' "$(date '+%Y/%m/%d %H:%M:%S')" "$1"; }
remaining=480
while kill -0 "$pid" 2>/dev/null; do
 if [ "$remaining" -le 0 ]; then report 'old process exit timed out'; exit 1; fi
 remaining=$((remaining - 1)); sleep 0.25
done
remaining=480
while :; do
 installed=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleShortVersionString' "$bundle/Contents/Info.plist" 2>/dev/null)
 if [ "$installed" = "$version" ]; then break; fi
 if [ "$remaining" -le 0 ]; then report 'installation timed out; app was not opened'; exit 1; fi
 remaining=$((remaining - 1)); sleep 0.25
done
/usr/bin/open -n "$bundle" --args --hoard-update-restart
status=$?
if [ "$status" -ne 0 ]; then report "open failed status=$status"; exit "$status"; fi
`

func macUpdateRestartCommand(pid int, bundle, version string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", macUpdateRestartScript, "hoard-update-restart", strconv.Itoa(pid), bundle, strings.TrimPrefix(version, "v"))
}

func restartForUpdate(version string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.HasSuffix(bundle, ".app") || filepath.Base(filepath.Dir(exe)) != "MacOS" {
		return fmt.Errorf("update restart requires an app bundle")
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(dir, "update-restart.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	cmd := macUpdateRestartCommand(os.Getpid(), bundle, version)
	cmd.Stdout, cmd.Stderr = file, file
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	// Quit lets the SDK install, without scheduling a second competing relaunch.
	mygo.App.Quit()
	return nil
}

func showUpdatedWindow(win *mygo.Window) {
	if !slices.Contains(os.Args, updateRestartFlag) {
		return
	}
	mygo.App.Show()
	win.Restore()
	win.Show()
	win.Focus()
}
