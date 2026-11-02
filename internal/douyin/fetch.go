package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Operation names a read endpoint the hidden page can call.
type Operation string

const (
	OpSelf      Operation = "self"
	OpLikes     Operation = "likes"
	OpFavorites Operation = "favorites"
	OpMusic     Operation = "music"
	OpDetail    Operation = "detail"
)

// PageReply is the raw answer of one in-page request.
type PageReply struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
	Error  string `json:"error"`
}

// pageKit is shared by the page scripts: the query fields the Douyin web
// client sends with every API call, and the msToken cookie the page SDK keeps
// fresh. Scripts embed it as an expression.
const pageKit = `(() => {
  const client = [
    ['device_platform', 'webapp'], ['aid', '6383'], ['channel', 'channel_pc_web'],
    ['version_code', '290100'], ['version_name', '29.1.0'], ['update_version_code', '170400'],
    ['cookie_enabled', 'true'], ['platform', 'PC']
  ];
  const readCookie = (name) => {
    try {
      for (const pair of document.cookie.split(';')) {
        const eq = pair.indexOf('=');
        if (eq > 0 && pair.slice(0, eq).trim() === name) return pair.slice(eq + 1).trim();
      }
    } catch {}
    return '';
  };
  const plain = (value) => { try { return decodeURIComponent(value); } catch { return value; } };
  return {
    // Later fields replace earlier ones in place, so the order stays the client's.
    query(fields) {
      const query = new URLSearchParams(client);
      for (const [key, value] of Object.entries(fields || {})) query.set(key, value);
      return query;
    },
    withToken(query) {
      const token = readCookie('msToken');
      if (token) query.set('msToken', plain(token));
      return query;
    }
  };
})()`

// pageRequestScript runs in the main world of the Douyin page, so the page's
// own security SDK wraps fetch and signs the request. Hoard never signs.
const pageRequestScript = `(async (job) => {
  const kit = ` + pageKit + `;
  const routes = {
    self: { path: '/aweme/v1/web/user/profile/self/' },
    likes: { path: '/aweme/v1/web/aweme/favorite/' },
    favorites: { path: '/aweme/v1/web/aweme/listcollection/', shelf: true, form: true },
    music: { path: '/aweme/v1/web/music/listcollection/', shelf: true },
    detail: { path: '/aweme/v1/web/aweme/detail/' }
  };
  const route = routes[job.op];
  const args = job.params || {};
  // A form request carries its paging in the body, not the query.
  const query = kit.query(route.form ? {} : args);
  if (job.op === 'detail') {
    query.set('aweme_id', args.aweme_id || '');
    query.set('pc_client_type', '1');
    query.set('publish_video_strategy_type', '2');
  }
  if (route.shelf) {
    // Collection lists also expect what the browser reports about itself.
    const mac = /Macintosh/.test(navigator.userAgent);
    if (!route.form && !query.has('count')) query.set('count', '20');
    const device = [
      ['pc_client_type', '1'],
      ['browser_language', navigator.language || ''],
      ['browser_platform', navigator.platform || ''],
      ['browser_online', String(navigator.onLine)],
      navigator.hardwareConcurrency && ['cpu_core_num', String(navigator.hardwareConcurrency)],
      mac && ['os_name', 'Mac OS'],
      mac && route.form && ['pc_libra_divert', 'Mac OS'],
      ['screen_width', String(screen.width)],
      ['screen_height', String(screen.height)],
      route.form && ['publish_video_strategy_type', '2']
    ];
    for (const field of device) if (field) query.set(field[0], field[1]);
  }
  if (job.op !== 'self') kit.withToken(query);
  const headers = { accept: 'application/json, text/plain, */*' };
  if (route.form) {
    headers['x-secsdk-csrf-token'] = 'DOWNGRADE';
    headers['content-type'] = 'application/x-www-form-urlencoded;charset=UTF-8';
  }
  const stop = new AbortController();
  const deadline = setTimeout(() => stop.abort(), 12000);
  const init = { method: route.form ? 'POST' : 'GET', credentials: 'include', cache: 'no-store', signal: stop.signal, headers };
  if (route.shelf) init.referrer = 'https://www.douyin.com/user/self?from_tab_name=main&showTab=favorite_collection';
  if (route.form) init.body = new URLSearchParams([['cursor', args.cursor || '0'], ['count', args.count || '20']]).toString();
  try {
    const reply = await fetch('https://www.douyin.com' + route.path + '?' + query, init);
    if (Number(reply.headers.get('content-length') || 0) > 16 * 1024 * 1024) return { status: reply.status, body: '', error: 'TOO_LARGE' };
    return { status: reply.status, body: await reply.text(), error: '' };
  } catch {
    return { status: 0, body: '', error: stop.signal.aborted ? 'TIMEOUT' : 'NETWORK_ERROR' };
  } finally {
    clearTimeout(deadline);
  }
})(%s)`

// pageVisible is a page-script function: whether an element is drawn on
// screen at a size a person could interact with.
const pageVisible = `((node) => {
  const box = node.getBoundingClientRect();
  if (box.width < 30 || box.height < 15) return false;
  if (box.bottom <= 0 || box.right <= 0 || box.top >= innerHeight || box.left >= innerWidth) return false;
  if (node.closest('[hidden], [inert], [aria-hidden="true"]')) return false;
  const style = getComputedStyle(node);
  return style.display !== 'none' && style.visibility !== 'hidden' && parseFloat(style.opacity || '1') > 0;
})`

// pageStateScript reports whether a verification challenge is showing: a
// challenge route, a visible challenge frame or container, or a visible dialog
// asking to verify. Scripts, hidden templates and fingerprint fields do not count.
const pageStateScript = `(() => {
  const shown = ` + pageVisible + `;
  const ready = document.readyState !== 'loading' && !!document.body;
  if (!ready) return { ready, captcha: false };
  const asks = /(拖动|按住|滑动).{0,12}(滑块|拼图|验证)|完成.{0,4}(安全)?验证|安全验证|旋转.{0,12}(图片|角度|验证)|slider captcha|security (check|verification)|verify (that )?you are human/i;
  const route = location.pathname.toLowerCase().split(/[\/_-]+/).some((part) => part === 'captcha' || part === 'verify' || part === 'verification');
  if (route) return { ready, captcha: true };
  const controls = 'input, button, label, select, textarea';
  const challenge = (node) => {
    if (node.matches(controls) || !shown(node)) return false;
    if (node.tagName === 'IFRAME') return /captcha|verify/i.test(node.getAttribute('src') || '');
    const name = (node.id + ' ' + (typeof node.className === 'string' ? node.className : '')).toLowerCase();
    return /secsdk|captcha[-_]?(verify|container|wrapper|box)|verify[-_]?captcha/.test(name) ||
      !!node.querySelector('[role="slider"], [class*="slider" i]') || asks.test(node.textContent || '');
  };
  const containers = document.querySelectorAll('iframe, [id*="captcha" i], [class*="captcha" i], [data-testid*="captcha" i]');
  for (const node of containers) if (challenge(node)) return { ready, captcha: true };
  for (const node of document.querySelectorAll('[role="dialog"], [aria-modal="true"], [id*="verify" i], [class*="verify" i]')) {
    if (shown(node) && asks.test(node.textContent || '')) return { ready, captcha: true };
  }
  return { ready, captcha: false };
})()`

// pageReadyScript tells whether the hidden page can run a request now: a
// loaded https Douyin document with fetch and timeouts, and no slider showing.
const pageReadyScript = `(() => {
  const shown = ` + pageVisible + `;
  const host = location.hostname;
  const sliders = document.querySelectorAll('iframe[src*="captcha" i], iframe[src*="verify" i], [role="slider"], [class*="secsdk" i], [class*="captcha_verify" i], [class*="captcha-verify" i]');
  return {
    douyin: host === 'douyin.com' || host.endsWith('.douyin.com'),
    secure: location.protocol === 'https:',
    loaded: document.readyState === 'interactive' || document.readyState === 'complete',
    fetch: typeof fetch === 'function',
    timeouts: typeof AbortController === 'function' && typeof setTimeout === 'function',
    slider: [...sliders].some(shown)
  };
})()`

// pageReadiness is the answer of pageReadyScript.
type pageReadiness struct {
	Douyin   bool `json:"douyin"`
	Secure   bool `json:"secure"`
	Loaded   bool `json:"loaded"`
	Fetch    bool `json:"fetch"`
	Timeouts bool `json:"timeouts"`
	Slider   bool `json:"slider"`
}

func (r pageReadiness) ok() bool {
	return r.Douyin && r.Secure && r.Loaded && r.Fetch && r.Timeouts && !r.Slider
}

// PageState tells whether the hidden page is usable and whether it shows a slider.
type PageState struct {
	Ready   bool `json:"ready"`
	Captcha bool `json:"captcha"`
}

// Request runs one signed read request in the account's hidden page.
func (a *Engine) Request(ctx context.Context, op Operation, params map[string]string) (PageReply, error) {
	page, err := a.readyPage(ctx)
	if err != nil {
		return PageReply{}, err
	}
	if params == nil {
		params = map[string]string{}
	}
	arg, err := json.Marshal(map[string]any{"op": op, "params": params})
	if err != nil {
		return PageReply{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	v, err := page.EvalContext(ctx, fmt.Sprintf(pageRequestScript, arg))
	if err != nil {
		return PageReply{}, err
	}
	return decode[PageReply](v)
}

// PageState inspects the hidden page without sending a request.
func (a *Engine) PageState(ctx context.Context) PageState {
	page, err := a.page()
	if err != nil {
		return PageState{}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	v, err := page.EvalContext(ctx, pageStateScript)
	if err != nil {
		return PageState{}
	}
	state, err := decode[PageState](v)
	if err != nil {
		return PageState{}
	}
	return state
}

// Recycle reloads the hidden page so the page SDK starts a fresh segment.
func (a *Engine) Recycle() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.douyin == nil {
		return errors.New("PAGE_NOT_READY")
	}
	a.domReady = false
	a.readyAt = time.Time{}
	return a.douyin.Page().LoadURL("https://www.douyin.com/user/self?from_tab_name=main&showTab=like")
}

// Visible reports whether the account window is on screen.
func (a *Engine) Visible() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.douyin != nil && a.douyin.IsVisible()
}
