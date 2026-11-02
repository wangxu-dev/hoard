package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

func decode[T any](v any) (T, error) {
	var out T
	b, err := json.Marshal(v)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

type ResponseClass string

const (
	ResponseOK                   ResponseClass = "OK"
	ResponseNotReady             ResponseClass = "NOT_READY"
	ResponseAuthRequired         ResponseClass = "AUTH_REQUIRED"
	ResponseVerificationRequired ResponseClass = "VERIFICATION_REQUIRED"
	ResponseRateLimited          ResponseClass = "RATE_LIMITED"
	ResponseRiskControl          ResponseClass = "RISK_CONTROL"
	ResponseAPIRejected          ResponseClass = "API_REJECTED"
)

func classifyResponse(operation string, httpStatus int, statusCode int64, statusMsg string, body string, envelopeOnly bool) ResponseClass {
	if httpStatus == 401 || httpStatus == 403 {
		return ResponseAuthRequired
	}
	if httpStatus == 429 {
		return ResponseRateLimited
	}
	lower := strings.ToLower(body + " " + statusMsg)
	if strings.HasPrefix(strings.TrimSpace(body), "<") || strings.Contains(lower, "captcha") || strings.Contains(lower, "verify_page") || strings.Contains(lower, "verify_center_decision_conf") || strings.Contains(statusMsg, "验证") || strings.Contains(statusMsg, "滑块") {
		return ResponseVerificationRequired
	}
	if operation == "self" {
		if statusCode == 0 && !envelopeOnly {
			return ResponseOK
		}
		return ResponseNotReady
	}
	if envelopeOnly || statusCode == 8 || statusCode == 2154 || statusCode == 2156 || statusCode == 10000 || statusCode == 10001 {
		return ResponseRiskControl
	}
	if statusCode != 0 {
		return ResponseAPIRejected
	}
	return ResponseOK
}
func (a *Engine) readyPage(ctx context.Context) (*mygo.Page, error) {
	page, err := a.page()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	warm := !a.readyAt.IsZero() && time.Since(a.readyAt) < time.Minute
	a.mu.Unlock()
	if warm {
		// The same document passed the full gate moments ago; one check is enough.
		quick, cancel := context.WithTimeout(ctx, 3*time.Second)
		v, evalErr := page.EvalContext(quick, pageReadyScript)
		cancel()
		if evalErr == nil {
			if f, decodeErr := decode[pageReadiness](v); decodeErr == nil && f.ok() {
				a.markReady(page)
				return page, nil
			}
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		deadline, cancel := context.WithTimeout(ctx, 30*time.Second)
		for deadline.Err() == nil {
			v, evalErr := page.EvalContext(deadline, pageReadyScript)
			if evalErr == nil {
				f, decodeErr := decode[pageReadiness](v)
				if decodeErr == nil && f.ok() {
					if !sleepContext(deadline, 1700*time.Millisecond) {
						break
					}
					v, evalErr = page.EvalContext(deadline, pageReadyScript)
					if evalErr == nil {
						f, decodeErr = decode[pageReadiness](v)
						if decodeErr == nil && f.ok() {
							cancel()
							a.markReady(page)
							return page, nil
						}
					}
				}
			}
			if !sleepContext(deadline, 200*time.Millisecond) {
				break
			}
		}
		cancel()
		if attempt == 0 {
			_ = page.LoadURL("https://www.douyin.com/")
			continue
		}
	}
	return nil, fmt.Errorf("PAGE_NOT_READY")
}

func (a *Engine) markReady(page *mygo.Page) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.douyin != nil && a.douyin.Page() == page && a.domReady {
		a.readyAt = time.Now()
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (a *Engine) CheckLogin() (LoginState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	identity, err := a.Identity(ctx)
	return identity.LoginState, err
}

func (a *Engine) Identity(ctx context.Context) (Identity, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	page, err := a.page()
	if err != nil {
		return Identity{}, err
	}
	value, err := page.EvalContext(ctx, pageReadyScript)
	if err != nil {
		return Identity{}, err
	}
	readiness, err := decode[pageReadiness](value)
	if err != nil || !readiness.ok() {
		return Identity{}, fmt.Errorf("PAGE_NOT_READY")
	}
	v, err := page.EvalContext(ctx, `(async () => {
  const kit = `+pageKit+`;
  try {
    const res = await fetch('https://www.douyin.com/aweme/v1/web/user/profile/self/?' + kit.query(), {
      signal: AbortSignal.timeout(12000),
      credentials: 'include',
      cache: 'no-store',
      headers: { accept: 'application/json, text/plain, */*' }
    });
    const text = await res.text();
    let data = null;
    try { data = JSON.parse(text); } catch {}
    const user = data && data.user;
    if (res.ok && data.status_code === 0 && user && user.sec_uid) {
      return {
        loggedIn: true,
        nickname: String(user.nickname || ''),
        uid: String(user.uid || user.sec_uid || ''),
        remoteId: String(user.sec_uid),
        avatar: String(user.avatar_thumb?.url_list?.[0] || ''),
        detail: ''
      };
    }
    return { loggedIn: false, nickname: '', uid: '', detail: 'IDENTITY_UNAVAILABLE', authRequired: res.status === 401 || res.status === 403 || (res.ok && data?.status_code === 0 && !user?.sec_uid) };
  } catch (e) {
    return { loggedIn: false, nickname: '', uid: '', detail: 'NETWORK_FAILED' };
  }
})()`)
	if err != nil {
		return Identity{}, err
	}
	state, err := decode[Identity](v)
	if err != nil {
		return Identity{}, err
	}
	return state, nil
}

func (a *Engine) FetchLikes(cursor string) (LikesResult, error) {
	return a.FetchLikesContext(context.Background(), cursor)
}

func (a *Engine) FetchLikesContext(ctx context.Context, cursor string) (LikesResult, error) {
	page, err := a.readyPage(ctx)
	if err != nil {
		return LikesResult{}, err
	}
	if cursor == "" {
		cursor = "0"
	}
	script := fmt.Sprintf(`(async () => {
  const kit = `+pageKit+`;
  try {
    const params = kit.withToken(kit.query({ count: '50', max_cursor: %q }));
    if (typeof navigator !== 'undefined') {
      params.set('browser_language', navigator.language || '');
      params.set('browser_platform', navigator.platform || '');
      const chrome = navigator.userAgent.match(/Chrome\/([\d.]+)/)?.[1];
      if (chrome) {
        params.set('browser_name', 'Chrome');
        params.set('browser_version', chrome);
        params.set('engine_name', 'Blink');
        params.set('engine_version', chrome);
      }
    }
    const res = await fetch('https://www.douyin.com/aweme/v1/web/aweme/favorite/?' + params, {
      signal: AbortSignal.timeout(18000),
      credentials: 'include',
      cache: 'no-store',
      headers: { accept: 'application/json, text/plain, */*' }
    });
    const text = await res.text();
    let data = null;
    try { data = JSON.parse(text); } catch {}
    const list = data && data.aweme_list;
    if (!res.ok || !data || data.status_code !== 0 || !Array.isArray(list)) {
      return {
        ok: false,
        items: [],
        cursor: '',
        hasMore: false,
        error: res.status === 401 || res.status === 403 ? 'AUTH_REQUIRED' : 'API_REJECTED'
      };
    }
    const firstUrl = (value) => {
      const urls = value && value.url_list;
      return Array.isArray(urls) && urls[0] ? String(urls[0]) : '';
    };
    const playOf = (item) => {
      const video = item && item.video;
      if (!video) return '';
      return firstUrl(video.play_addr_h264) || firstUrl(video.play_addr) || firstUrl(video.download_addr) || '';
    };
    const coverOf = (item) => {
      const video = item && item.video;
      const cover = video && (firstUrl(video.cover) || firstUrl(video.origin_cover) || firstUrl(video.dynamic_cover));
      if (cover) return cover;
      const images = Array.isArray(item && item.images) ? item.images : [];
      return images[0] ? firstUrl(images[0]) : '';
    };
    const imagesOf = (item) => {
      const images = Array.isArray(item && item.images) ? item.images : [];
      return images.map(firstUrl).filter(Boolean);
    };
    const items = list.map((item) => {
      const id = String((item && (item.aweme_id || item.id)) || '');
      return {
        id,
        desc: String((item && (item.desc || item.title)) || ''),
        author: item && item.author ? String(item.author.nickname || '') : '',
        cover: coverOf(item),
        duration: Number(item && item.video && item.video.duration) || 0,
        createdAt: Number(item && item.create_time) || 0,
        awemeType: Number(item && item.aweme_type) || 0,
        playUrl: playOf(item),
        images: imagesOf(item)
      };
    });
    return {
      ok: true,
      items,
      cursor: String(data.max_cursor || data.cursor || ''),
      hasMore: !!(data.has_more),
      error: ''
    };
  } catch (e) {
    return { ok: false, items: [], cursor: '', hasMore: false, error: 'NETWORK_FAILED' };
  }
})()`, cursor)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	v, err := page.EvalContext(ctx, script)
	if err != nil {
		return LikesResult{}, err
	}
	result, err := decode[LikesResult](v)
	if err != nil {
		return LikesResult{}, err
	}
	return result, nil
}
