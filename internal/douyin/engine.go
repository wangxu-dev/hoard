package douyin

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type Engine struct {
	mu       sync.Mutex
	douyin   *mygo.Window
	sess     *mygo.Session
	domReady bool
	// readyAt is when the current document last passed the full readiness gate.
	readyAt time.Time
}

func (a *Engine) UseSession(name string, show bool) error {
	if name == "" {
		return fmt.Errorf("SESSION_REQUIRED")
	}
	a.Close()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sess = mygo.NewSession(name)
	a.openWindowLocked(show)
	return nil
}

func (a *Engine) Close() {
	a.mu.Lock()
	win := a.douyin
	a.douyin = nil
	a.domReady = false
	a.mu.Unlock()
	if win != nil {
		win.Close()
	}
}

func (a *Engine) ClearSession(name string) error {
	return mygo.NewSession(name).ClearData(mygo.SiteDataAll)
}

func (a *Engine) openWindowLocked(show bool) Status {
	if a.douyin != nil {
		if show {
			a.douyin.Show()
			a.douyin.Focus()
		}
		return a.statusLocked()
	}
	log.Printf("[hoard:douyin] login window create visible=%t", show)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     "抖音登录",
		Width:     1100,
		Height:    800,
		MinWidth:  640,
		MinHeight: 480,
		Hidden:    !show,
		URL:       "about:blank",
		Page: mygo.PageOptions{
			Untrusted:       true,
			PreloadScript:   `document.addEventListener('play', e => { if (e.target instanceof HTMLMediaElement) { e.target.muted = true; e.target.pause(); } }, true);`,
			Session:         a.sess,
			SwipeNavigation: true,
		},
	})
	win.Page().OnWillNavigate(func(event *mygo.NavigateEvent) {
		if !douyinNavigationAllowed(event.URL) {
			log.Printf("[hoard:douyin] navigation refused page=%s", loginPageClass(event.URL))
			event.PreventDefault()
		}
	})
	win.Page().OnRenderProcessGone(func(_ string) { log.Print("[hoard:douyin] web content process exited") })
	win.Page().OnDidNavigate(func(raw string) { log.Printf("[hoard:douyin] navigation committed page=%s", loginPageClass(raw)) })
	win.Page().OnDidFinishLoad(func() { log.Print("[hoard:douyin] page load finished") })
	win.Page().OnDidFailLoad(func(err *mygo.LoadError) { log.Printf("[hoard:douyin] page load failed code=%d", err.Code) })
	win.Page().SetAudioMuted(true)
	a.domReady = false
	a.readyAt = time.Time{}
	win.Page().OnDOMReady(func() {
		log.Print("[hoard:douyin] DOM ready")
		parsed, err := url.Parse(win.Page().URL())
		if err != nil || parsed.Scheme != "https" || (parsed.Hostname() != "douyin.com" && !strings.HasSuffix(parsed.Hostname(), ".douyin.com")) {
			return
		}
		a.mu.Lock()
		if a.douyin == win {
			a.domReady = true
			a.readyAt = time.Time{}
		}
		a.mu.Unlock()
	})
	win.OnClosed(func() {
		a.mu.Lock()
		if a.douyin == win {
			a.douyin = nil
		}
		a.mu.Unlock()
	})
	a.douyin = win
	if err := win.Page().LoadURL("https://www.douyin.com/user/self"); err != nil {
		log.Print("[hoard:douyin] initial navigation request failed")
	} else {
		log.Print("[hoard:douyin] initial navigation requested")
	}
	return a.statusLocked()
}

func douyinNavigationAllowed(raw string) bool {
	if raw == "about:blank" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (u.Hostname() == "douyin.com" || strings.HasSuffix(u.Hostname(), ".douyin.com"))
}

// loginPageClass records only a fixed category, never account paths or query values.
func loginPageClass(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid"
	}
	if u.Scheme == "about" {
		return "blank"
	}
	if u.Scheme != "https" || (u.Hostname() != "douyin.com" && !strings.HasSuffix(u.Hostname(), ".douyin.com")) {
		return "external"
	}
	switch u.Path {
	case "/":
		return "home"
	case "/user/self":
		return "self"
	}
	return "douyin_other"
}

func (a *Engine) OpenDouyin() (Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sess == nil {
		return Status{}, fmt.Errorf("SESSION_REQUIRED")
	}
	return a.openWindowLocked(true), nil
}

func (a *Engine) EnsureSession() (Status, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sess == nil {
		return Status{}, fmt.Errorf("SESSION_REQUIRED")
	}
	return a.openWindowLocked(false), nil
}

func (a *Engine) CookiePresent() bool {
	a.mu.Lock()
	sess := a.sess
	a.mu.Unlock()
	if sess == nil {
		return false
	}
	cookies, err := sess.Cookies.List(mygo.CookieFilter{URL: "https://www.douyin.com/"})
	if err != nil {
		return false
	}
	for _, cookie := range cookies {
		switch strings.ToLower(cookie.Name) {
		case "sessionid", "sessionid_ss", "sid_tt", "uid_tt":
			return true
		}
	}
	return false
}

func (a *Engine) ShowDouyin() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.douyin == nil {
		return fmt.Errorf("未打开抖音")
	}
	a.douyin.Show()
	a.douyin.Focus()
	return nil
}

func (a *Engine) HideDouyin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.douyin != nil {
		a.douyin.Hide()
	}
}

func (a *Engine) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *Engine) statusLocked() Status {
	if a.douyin == nil {
		return Status{}
	}
	page := a.douyin.Page()
	return Status{Open: true, Ready: a.domReady, URL: page.URL(), Loading: page.IsLoading()}
}

func (a *Engine) page() (*mygo.Page, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.douyin == nil || !a.domReady {
		return nil, fmt.Errorf("PAGE_NOT_READY")
	}
	return a.douyin.Page(), nil
}
