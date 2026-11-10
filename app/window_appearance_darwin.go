//go:build darwin

package app

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var chromeColor func(objc.ID, objc.SEL, float64, float64, float64, float64) objc.ID
var chromeApp, chromeWindow, chromeObserver objc.ID

func chromeSend(object objc.ID, selector string, args ...any) objc.ID {
	return object.Send(objc.RegisterName(selector), args...)
}
func chromeString(text string) objc.ID {
	return chromeSend(objc.ID(objc.GetClass("NSString")), "stringWithUTF8String:", text)
}
func updateChromeContentAppearance() {
	windows := chromeSend(chromeApp, "windows")
	appearance := chromeSend(chromeApp, "effectiveAppearance")
	for i := 0; i < int(chromeSend(windows, "count")); i++ {
		window := chromeSend(windows, "objectAtIndex:", i)
		if window == chromeWindow {
			chromeSend(chromeSend(window, "contentView"), "setAppearance:", appearance)
		}
	}
}

// This is the same existing pure-Go Objective-C runtime used by mygo itself.
// It must work with CGO_ENABLED=0, which is how mygo builds native releases.
func setNativeTitlebar(handle uintptr, dark bool, r, g, b uint8) {
	if handle == 0 {
		return
	}
	if chromeColor == nil {
		lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&chromeColor, lib, "objc_msgSend")
		chromeApp = chromeSend(objc.ID(objc.GetClass("NSApplication")), "sharedApplication")
		cls, err := objc.RegisterClass("HoardChromeSystemAppearanceV1", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{Cmd: objc.RegisterName("observeValueForKeyPath:ofObject:change:context:"), Fn: func(self objc.ID, cmd objc.SEL, keyPath, object, change objc.ID, context uintptr) {
			if object == chromeApp {
				updateChromeContentAppearance()
			}
		}}})
		if err == nil {
			chromeObserver = chromeSend(chromeSend(objc.ID(cls), "alloc"), "init")
			chromeSend(chromeApp, "addObserver:forKeyPath:options:context:", chromeObserver, chromeString("effectiveAppearance"), uint64(1), uintptr(0))
		}
	}
	window := objc.ID(handle)
	chromeWindow = window
	// Keep WebKit's media query tied to real system appearance, not this window's
	// deliberately dark galactic chrome. The observer maintains it on OS changes.
	chromeSend(chromeSend(window, "contentView"), "setAppearance:", chromeSend(chromeApp, "effectiveAppearance"))
	name := "NSAppearanceNameAqua"
	if dark {
		name = "NSAppearanceNameDarkAqua"
	}
	appearance := chromeSend(objc.ID(objc.GetClass("NSAppearance")), "appearanceNamed:", chromeString(name))
	chromeSend(window, "setAppearance:", appearance)
	chromeSend(window, "setTitlebarAppearsTransparent:", true)
	color := chromeColor(objc.ID(objc.GetClass("NSColor")), objc.RegisterName("colorWithSRGBRed:green:blue:alpha:"), float64(r)/255, float64(g)/255, float64(b)/255, 1)
	chromeSend(window, "setBackgroundColor:", color)
}
