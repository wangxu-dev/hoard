package source

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/wangxu-dev/hoard/internal/douyin"
)

// Engine routes account windows while retaining the existing Douyin request API.
type Engine struct {
	*douyin.Engine
	mu      sync.Mutex
	source  string
	session string
	window  *mygo.Window
	ready   bool
}

func NewEngine() *Engine         { return &Engine{Engine: &douyin.Engine{}, source: "douyin"} }
func (e *Engine) Source() string { e.mu.Lock(); defer e.mu.Unlock(); return e.source }
func (e *Engine) UseSession(name string, show bool) error {
	return e.UseSourceSession("douyin", name, show)
}
func (e *Engine) UseSourceSession(source, name string, show bool) error {
	if source != "douyin" && source != "bilibili" {
		return errors.New("UNSUPPORTED_SOURCE")
	}
	if name == "" {
		return errors.New("SESSION_REQUIRED")
	}
	e.Close()
	e.mu.Lock()
	e.source, e.session = source, name
	e.mu.Unlock()
	if source == "douyin" {
		return e.Engine.UseSession(name, show)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.openLocked(show)
}
func (e *Engine) openLocked(show bool) error {
	if e.window != nil {
		if show {
			e.window.Show()
			e.window.Focus()
		}
		return nil
	}
	win := mygo.NewWindow(mygo.WindowOptions{Title: "Hoard · Bilibili", Width: 1100, Height: 800, Hidden: !show, URL: "about:blank", Page: mygo.PageOptions{Untrusted: true, Session: mygo.NewSession(e.session)}})
	win.Page().SetUserAgent(win.Page().UserAgent() + " Version/27.0 Safari/605.1.15")
	win.Page().SetAudioMuted(true)
	win.Page().OnWillNavigate(func(event *mygo.NavigateEvent) {
		if !bilibiliURL(event.URL) {
			event.PreventDefault()
		}
	})
	win.Page().OnDOMReady(func() {
		e.mu.Lock()
		if e.window == win {
			e.ready = true
		}
		e.mu.Unlock()
	})
	win.OnClosed(func() {
		e.mu.Lock()
		if e.window == win {
			e.window = nil
			e.ready = false
		}
		e.mu.Unlock()
	})
	e.window = win
	e.ready = false
	return win.Page().LoadURL("https://www.bilibili.com/")
}
func bilibiliURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (u.Hostname() == "bilibili.com" || strings.HasSuffix(u.Hostname(), ".bilibili.com"))
}
func (e *Engine) Close() {
	e.Engine.Close()
	e.mu.Lock()
	win := e.window
	e.window = nil
	e.ready = false
	e.mu.Unlock()
	if win != nil {
		win.Close()
	}
}
func (e *Engine) Status() douyin.Status {
	if e.Source() == "douyin" {
		return e.Engine.Status()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.window == nil {
		return douyin.Status{}
	}
	return douyin.Status{Open: true, Ready: e.ready, URL: e.window.Page().URL(), Loading: e.window.Page().IsLoading()}
}
func (e *Engine) EnsureSession() (douyin.Status, error) {
	if e.Source() == "douyin" {
		return e.Engine.EnsureSession()
	}
	e.mu.Lock()
	err := e.openLocked(false)
	e.mu.Unlock()
	return e.Status(), err
}
func (e *Engine) OpenDouyin() (douyin.Status, error) {
	if e.Source() == "douyin" {
		return e.Engine.OpenDouyin()
	}
	e.mu.Lock()
	err := e.openLocked(true)
	e.mu.Unlock()
	return e.Status(), err
}
func (e *Engine) ShowDouyin() error { _, err := e.OpenDouyin(); return err }
func (e *Engine) HideDouyin() {
	if e.Source() == "douyin" {
		e.Engine.HideDouyin()
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.window != nil {
		e.window.Hide()
	}
}
func (e *Engine) SaveLogin(ctx context.Context) bool {
	if e.Source() == "douyin" {
		return e.Engine.SaveLogin(ctx)
	}
	return ctx.Err() == nil
}
func (e *Engine) Identity(ctx context.Context) (douyin.Identity, error) {
	if e.Source() == "douyin" {
		return e.Engine.Identity(ctx)
	}
	e.mu.Lock()
	win, ready := e.window, e.ready
	e.mu.Unlock()
	if win == nil || !ready {
		return douyin.Identity{}, errors.New("NOT_READY")
	}
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := win.Page().EvalContext(call, bilibiliIdentityScript)
	if err != nil {
		return douyin.Identity{}, errors.New("IDENTITY_FAILED")
	}
	body, err := json.Marshal(value)
	if err != nil {
		return douyin.Identity{}, err
	}
	return parseBilibiliIdentity(body)
}
func parseBilibiliIdentity(body []byte) (douyin.Identity, error) {
	var result struct {
		Code *int `json:"code"`
		Data struct {
			LoggedIn bool        `json:"isLogin"`
			ID       json.Number `json:"mid"`
			Name     string      `json:"uname"`
			Avatar   string      `json:"face"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Code == nil {
		return douyin.Identity{}, errors.New("INVALID_IDENTITY")
	}
	if *result.Code == -101 || (*result.Code == 0 && !result.Data.LoggedIn) {
		return douyin.Identity{AuthRequired: true}, nil
	}
	if *result.Code != 0 {
		return douyin.Identity{}, errors.New("IDENTITY_FAILED")
	}
	id := string(result.Data.ID)
	if id == "" || id == "0" || strings.Trim(id, "0123456789") != "" {
		return douyin.Identity{}, errors.New("INVALID_IDENTITY")
	}
	return douyin.Identity{LoginState: douyin.LoginState{LoggedIn: true, UID: id, Nickname: result.Data.Name}, RemoteID: id, Avatar: result.Data.Avatar}, nil
}
func (e *Engine) Request(ctx context.Context, op douyin.Operation, params map[string]string) (douyin.PageReply, error) {
	if e.Source() != "douyin" {
		return douyin.PageReply{}, errors.New("UNSUPPORTED_SOURCE")
	}
	return e.Engine.Request(ctx, op, params)
}
func (e *Engine) FetchLikesContext(ctx context.Context, cursor string) (douyin.LikesResult, error) {
	if e.Source() != "douyin" {
		return douyin.LikesResult{}, errors.New("UNSUPPORTED_SOURCE")
	}
	return e.Engine.FetchLikesContext(ctx, cursor)
}

const bilibiliIdentityScript = `(async()=>{
 if(location.hostname!=='www.bilibili.com') return {code:-1};
 const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),10000);
 try{
  const response=await fetch('https://api.bilibili.com/x/web-interface/nav',{credentials:'include',signal:controller.signal});
  if(!response.ok)return {code:-1};
  const result=await response.json();
  if(result.code===-101||(result.code===0&&!result.data?.isLogin)){
   if(!window.__hoardLoginOpened&&!document.querySelector('.bili-mini-login-right-wp')){
    const entry=document.querySelector('.header-avatar-unlogin-entry');
    if(entry){entry.dispatchEvent(new MouseEvent('mouseenter',{bubbles:true}));await new Promise(resolve=>setTimeout(resolve,300));const button=document.querySelector('.login-btn');if(button&&button.textContent.trim()==='立即登录'){button.click();window.__hoardLoginOpened=true;}}
   }
  }
  return {code:result.code,data:result.data?{isLogin:result.data.isLogin,mid:String(result.data.mid||''),uname:result.data.uname,face:result.data.face}:null};
 }finally{clearTimeout(timer)}
})()`
