package app

import (
	"github.com/egoist/mygo"
)

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
	win.Page().OnContextMenu(func(event *mygo.ContextMenuEvent) {
		if !event.Editable && event.SelectionText == "" {
			event.PreventDefault()
			return
		}
		for _, item := range event.Menu.Items() {
			switch item.ID {
			case mygo.ContextMenuUndo, mygo.ContextMenuRedo, mygo.ContextMenuCut, mygo.ContextMenuCopy, mygo.ContextMenuPaste, mygo.ContextMenuPasteAndMatchStyle, mygo.ContextMenuDelete, mygo.ContextMenuSelectAll:
			default:
				item.Hidden = true
			}
		}
	})
	win.OnReadyToShow(func() {
		showUpdatedWindow(win)
	})
	win.OnClosed(mygo.App.Quit)
	return win
}
