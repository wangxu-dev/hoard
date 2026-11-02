package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ReadBilibili only accepts the read endpoints used by the account indexer.
func (e *Engine) ReadBilibili(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	switch path {
	case "/x/web-interface/view", "/x/player/playurl", "/x/v2/history/toview", "/x/v3/fav/folder/created/list-all", "/x/v3/fav/resource/list", "/x/web-interface/archive/has/like", "/x/v2/fav/video/favoured", "/x/web-interface/archive/coins":
	default:
		return nil, errors.New("UNSUPPORTED_ENDPOINT")
	}
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
	}{path, params})
	if err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	value, err := win.Page().EvalContext(call, fmt.Sprintf(`(async(job)=>{
 if(location.hostname!=='www.bilibili.com')return {error:'NOT_READY'};
 const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),10000);
 try{const response=await fetch('https://api.bilibili.com'+job.path+'?'+new URLSearchParams(job.params||{}),{credentials:'include',signal:controller.signal});
 if(!response.ok)return {error:response.status===429?'RATE_LIMITED':'REQUEST_FAILED'};
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
			return nil, errors.New(reply.Error)
		}
		return nil, errors.New("INVALID_RESPONSE")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []byte(reply.Body), nil
}
