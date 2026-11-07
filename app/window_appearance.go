package app

import (
	"errors"
	"github.com/egoist/mygo"
	"strconv"
)

// WindowAppearance controls only the main window, never the application/system theme.
type WindowAppearance struct{ window func() *mygo.Window }

func NewWindowAppearance(window func() *mygo.Window) *WindowAppearance {
	return &WindowAppearance{window: window}
}
func parseWindowColor(color string) (uint8, uint8, uint8, error) {
	if len(color) != 7 || color[0] != '#' {
		return 0, 0, 0, errors.New("INVALID_WINDOW_COLOR")
	}
	n, err := strconv.ParseUint(color[1:], 16, 24)
	if err != nil {
		return 0, 0, 0, errors.New("INVALID_WINDOW_COLOR")
	}
	return uint8(n >> 16), uint8(n >> 8), uint8(n), nil
}
func (a *WindowAppearance) Apply(dark bool, color string) error {
	r, g, b, err := parseWindowColor(color)
	if err != nil {
		return err
	}
	mygo.RunOnMain(func() {
		win := a.window()
		if win == nil || win.IsDestroyed() {
			return
		}
		_ = win.SetBackgroundColor(color)
		setNativeTitlebar(win.NativeHandle(), dark, r, g, b)
	})
	return nil
}
