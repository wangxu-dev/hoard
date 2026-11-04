package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
)

// ReadBilibili only accepts the read endpoints used by the account indexer.
func (e *Engine) ReadBilibili(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	switch path {
	case "/x/polymer/web-dynamic/v1/opus/feed/fav", "/x/polymer/web-dynamic/v1/opus/detail", "/x/web-interface/view", "/x/player/playurl", "/x/v2/history/toview", "/x/v3/fav/folder/created/list-all", "/x/v3/fav/resource/list", "/x/web-interface/archive/has/like", "/x/v2/fav/video/favoured", "/x/web-interface/archive/coins":
	default:
		return nil, errors.New("UNSUPPORTED_ENDPOINT")
	}
	return e.requestBilibili(ctx, path, params, false)
}

// WriteBilibili keeps credentials and CSRF inside the account page.
func (e *Engine) WriteBilibili(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	switch path {
	case "/x/web-interface/archive/like", "/x/v3/fav/resource/deal", "/x/v2/history/toview/add", "/x/v2/history/toview/del", "/dynamic_like/v1/dynamic_like/thumb", "/x/community/cosmo/interface/simple_action":
	default:
		return nil, errors.New("UNSUPPORTED_ENDPOINT")
	}
	return e.requestBilibili(ctx, path, params, true)
}

func (e *Engine) requestBilibili(ctx context.Context, path string, params map[string]string, write bool) ([]byte, error) {
	e.mu.Lock()
	win, ready, source := e.window, e.ready, e.source
	e.mu.Unlock()
	if source != "bilibili" || win == nil || !ready {
		return nil, errors.New("NOT_READY")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	job, err := json.Marshal(struct {
		Path   string            `json:"path"`
		Params map[string]string `json:"params"`
		Write  bool              `json:"write"`
	}{path, params, write})
	if err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := win.Page().EvalContext(call, fmt.Sprintf(`(async(job)=>{
 if(location.hostname!=='www.bilibili.com')return {error:'NOT_READY'};
 const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),10000);
 try{
 const host=job.path==='/dynamic_like/v1/dynamic_like/thumb'?'https://api.vc.bilibili.com':'https://api.bilibili.com';
 const url=new URL(host+job.path);const options={credentials:'include',signal:controller.signal};
 if(job.write){
  const csrf=document.cookie.split(';').map(v=>v.trim()).find(v=>v.startsWith('bili_jct='))?.slice(9);
  if(!csrf)return {error:'AUTH_REQUIRED'};
  options.method='POST';
  if(job.path==='/x/community/cosmo/interface/simple_action'){
   url.searchParams.set('csrf',decodeURIComponent(csrf));options.headers={'Content-Type':'application/json'};
   options.body=JSON.stringify({meta:{spmid:'444.42.0.0',from_spmid:'333.1365.0.0',from:'unknown'},entity:{object_id_str:job.params.id,type:{biz:2}},action:Number(job.params.action)});
  }else{options.headers={'Content-Type':'application/x-www-form-urlencoded'};options.body=new URLSearchParams({...job.params,csrf:decodeURIComponent(csrf),csrf_token:decodeURIComponent(csrf)}).toString()}
 }else{url.search=new URLSearchParams(job.params||{}).toString()}
 const response=await fetch(url,options);
 if(!response.ok)return {error:response.status===429?'RATE_LIMITED':response.status===412||response.status===403?'RISK_CONTROL':response.status===401?'AUTH_REQUIRED':'REQUEST_FAILED'};
 const body=await response.text();return body.length<=16777216?{body}:{error:'RESPONSE_TOO_LARGE'};
 }finally{clearTimeout(timer)}
})(%s)`, job))
	if err != nil {
		return nil, errors.New("REQUEST_FAILED")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Body  string `json:"body"`
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &reply) != nil || reply.Body == "" {
		if reply.Error != "" {
			switch reply.Error {
			case "RATE_LIMITED":
				return nil, bilibili.ErrRateLimited
			case "RISK_CONTROL":
				return nil, bilibili.ErrRiskControl
			case "AUTH_REQUIRED":
				return nil, bilibili.ErrAuth
			}
			return nil, errors.New(reply.Error)
		}
		return nil, errors.New("INVALID_RESPONSE")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte(reply.Body), nil
}
