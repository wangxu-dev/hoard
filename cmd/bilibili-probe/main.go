package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
)

// This isolated diagnostic never opens Hoard's library or exports session data.
func main() {
	output := flag.String("log", ".agent/logs/bilibili-probe.log", "sanitized diagnostic log")
	flag.Parse()
	file, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal("cannot open diagnostic log")
	}
	defer file.Close()
	log.SetOutput(file)
	mygo.App.SetName("Hoard Bilibili Probe")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{Title: "Hoard · B站登录验证", Width: 1100, Height: 800, URL: "about:blank", Page: mygo.PageOptions{Untrusted: true, Session: mygo.NewSession("hoard-bilibili-probe")}})
		win.Page().SetUserAgent(win.Page().UserAgent() + " Version/27.0 Safari/605.1.15")
		win.Page().OnWillNavigate(func(event *mygo.NavigateEvent) {
			u, err := url.Parse(event.URL)
			if err != nil || u.Scheme != "https" || !(u.Hostname() == "bilibili.com" || strings.HasSuffix(u.Hostname(), ".bilibili.com")) {
				event.PreventDefault()
				log.Print("[hoard:bilibili-probe] navigation blocked")
				return
			}
			log.Printf("[hoard:bilibili-probe] navigating host=%s", u.Hostname())
		})
		win.Page().OnDidFailLoad(func(err *mygo.LoadError) { log.Printf("[hoard:bilibili-probe] load failed code=%d", err.Code) })
		win.Page().OnDidFinishLoad(func() {
			log.Print("[hoard:bilibili-probe] load finished")
			go func() {
				c, stop := context.WithTimeout(ctx, 5*time.Second)
				defer stop()
				value, err := win.Page().EvalContext(c, `JSON.stringify({host:location.hostname,scheme:location.protocol,userAgent:navigator.userAgent,elements:document.body?.childElementCount||0,textLength:document.body?.innerText.length||0})`)
				if err == nil {
					log.Printf("[hoard:bilibili-probe] page shape %v", value)
				}
			}()
		})
		win.Page().SetAudioMuted(true)
		win.OnClosed(func() { cancel(); mygo.App.Quit() })
		var ready atomic.Bool
		win.Page().OnDOMReady(func() { ready.Store(true); log.Print("[hoard:bilibili-probe] DOM ready") })
		if err := win.Page().LoadURL("https://www.bilibili.com/"); err != nil {
			log.Print("[hoard:bilibili-probe] login navigation failed")
		}
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			log.Print("[hoard:bilibili-probe] waiting for login")
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				if !ready.Load() {
					continue
				}
				callCtx, stop := context.WithTimeout(ctx, 60*time.Second)
				value, err := win.Page().EvalContext(callCtx, probeScript)
				stop()
				if err != nil {
					log.Print("[hoard:bilibili-probe] page unavailable; waiting")
					continue
				}
				bytes, err := json.Marshal(value)
				if err != nil {
					continue
				}
				var result probeResult
				if json.Unmarshal(bytes, &result) != nil {
					log.Print("[hoard:bilibili-probe] invalid summary")
					continue
				}
				// Only typed summaries reach the log, even if the page returned extra fields.
				if result.State == "waiting" {
					continue
				}
				if result.State != "complete" && result.State != "failed" {
					continue
				}
				valid := true
				for _, item := range result.Results {
					switch item.Kind {
					case "identity", "folders", "items", "detail", "play":
					default:
						valid = false
					}
				}
				if !valid {
					continue
				}
				clean, _ := json.Marshal(result)
				log.Printf("[hoard:bilibili-probe] %s", clean)
				return
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		log.Print("[hoard:bilibili-probe] application stopped")
	}
}

type endpointSummary struct {
	Kind    string `json:"kind"`
	Status  int    `json:"status"`
	Code    int    `json:"code"`
	Count   int    `json:"count"`
	HasMore bool   `json:"hasMore"`
	Video   bool   `json:"video"`
	Audio   bool   `json:"audio"`
}
type probeResult struct {
	State   string            `json:"state"`
	Results []endpointSummary `json:"results"`
}

const probeScript = `(async () => {
if (location.hostname !== 'www.bilibili.com') return {state:'waiting',results:[]};
 const results = [];
 const read = async (kind, path, params = {}) => {
  const stop = new AbortController();
  const timer = setTimeout(() => stop.abort(), 10000);
  const summary = {kind, status: 0, code: -1, count: 0, hasMore: false, video: false, audio: false};
  results.push(summary);
  try {
   const response = await fetch('https://api.bilibili.com' + path + '?' + new URLSearchParams(params), {credentials:'include', signal:stop.signal});
   summary.status = response.status;
   const data = await response.json();
   summary.code = Number.isInteger(data.code) ? data.code : -1;
   return {data:data.data, summary};
  } catch { return {data:null, summary}; } finally { clearTimeout(timer); }
 };
 const identity = await read('identity','/x/web-interface/nav');
 if (identity.summary.code === -101 || (identity.summary.code === 0 && !identity.data?.isLogin)) {
  if (!window.__hoardLoginOpened && !document.querySelector('.bili-mini-login-right-wp')) {
   const entry = document.querySelector('.header-avatar-unlogin-entry');
   if (entry) {
    entry.dispatchEvent(new MouseEvent('mouseenter', {bubbles:true}));
    await new Promise(resolve => setTimeout(resolve, 300));
    const button = document.querySelector('.login-btn');
    if (button && button.textContent.trim() === '立即登录') {button.click();window.__hoardLoginOpened=true;}
   }
  }
  return {state:'waiting',results:[]};
 }
 if (identity.summary.code !== 0 || !identity.data?.mid) return {state:'failed',results};
 const folders = await read('folders','/x/v3/fav/folder/created/list-all',{up_mid:String(identity.data.mid)});
 if (folders.summary.code !== 0) return {state:'failed',results};
 const list = Array.isArray(folders.data?.list) ? folders.data.list : [];
 folders.summary.count = list.length;
 if (!list.length) return {state:'complete',results};
 const folder = list.find(item => item.media_count > 0) || list[0];
 const items = await read('items','/x/v3/fav/resource/list',{media_id:String(folder.id),pn:'1',ps:'20',order:'mtime',type:'0'});
 if (items.summary.code !== 0) return {state:'failed',results};
 const media = Array.isArray(items.data?.medias) ? items.data.medias : [];
 items.summary.count = media.length;
 items.summary.hasMore = !!items.data?.has_more;
 const video = media.find(item => item.type === 2 && /^BV[0-9A-Za-z]+$/.test(item.bvid || ''));
 if (!video) return {state:'complete',results};
 const detail = await read('detail','/x/web-interface/view',{bvid:video.bvid});
 detail.summary.count = Array.isArray(detail.data?.pages) ? detail.data.pages.length : 0;
 if (detail.summary.code !== 0 || !detail.data?.cid) return {state:'failed',results};
 const play = await read('play','/x/player/playurl',{bvid:video.bvid,cid:String(detail.data.cid),qn:'16',fnval:'16',fourk:'0'});
 play.summary.video = !!(play.data?.dash?.video?.length || play.data?.durl?.length);
 play.summary.audio = !!play.data?.dash?.audio?.length;
 return {state:play.summary.code === 0 ? 'complete':'failed',results};
})()`
