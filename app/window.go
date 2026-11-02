package app

import "github.com/egoist/mygo"

func NewWindow() *mygo.Window {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     "Hoard",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		URL:       "/",
		StateKey:  "main",
	})
	win.OnClosed(mygo.App.Quit)
	return win
}
