package douyin

import (
	"strings"
	"testing"
)

func TestParseListLikes(t *testing.T) {
	body := `{"status_code":0,"has_more":1,"max_cursor":1700000000000,"aweme_list":[{"aweme_id":"7300000000000000001","desc":"hi","create_time":1700000000,"author":{"nickname":"a","sec_uid":"MS4wLjABAAAA"},"video":{"duration":15000,"cover":{"url_list":["https://p3.douyinpic.com/c.jpg"]}}},{"aweme_id":"7300000000000000002","desc":"g","images":[{"url_list":["https://p3.douyinpic.com/i.jpg"]}]}]}`
	p := ParseList(OpLikes, PageReply{Status: 200, Body: body})
	if p.Class != ResponseOK || len(p.Works) != 2 || !p.HasMore || p.Cursor != "1700000000000" {
		t.Fatalf("unexpected page: %+v", p)
	}
	if p.Works[0].Kind != "video" || p.Works[0].Cover == "" || p.Works[1].Kind != "gallery" || p.Works[1].Cover == "" {
		t.Fatalf("unexpected works: %+v", p.Works)
	}
}

func TestParseListClasses(t *testing.T) {
	cases := map[string]struct {
		op    Operation
		reply PageReply
		want  ResponseClass
	}{
		"rate":     {OpLikes, PageReply{Status: 429}, ResponseRateLimited},
		"html":     {OpLikes, PageReply{Status: 200, Body: "<html>"}, ResponseVerificationRequired},
		"risk":     {OpLikes, PageReply{Status: 200, Body: `{"status_code":2154}`}, ResponseRiskControl},
		"empty":    {OpFavorites, PageReply{Status: 200, Body: ""}, ResponseRiskControl},
		"envelope": {OpLikes, PageReply{Status: 200, Body: `{"status_code":0,"extra":{}}`}, ResponseRiskControl},
		"captcha":  {OpLikes, PageReply{Status: 200, Body: `{"status_code":0,"verify_page":{"a":1}}`}, ResponseVerificationRequired},
		"end":      {OpFavorites, PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"cursor":0}`}, ResponseOK},
		"rejected": {OpMusic, PageReply{Status: 200, Body: `{"status_code":5}`}, ResponseAPIRejected},
		"network":  {OpLikes, PageReply{Error: "TIMEOUT"}, ResponseNotReady},
	}
	for name, c := range cases {
		if got := ParseList(c.op, c.reply).Class; got != c.want {
			t.Errorf("%s: got %s want %s", name, got, c.want)
		}
	}
}

func TestParseMusicAndProfile(t *testing.T) {
	p := ParseList(OpMusic, PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"cursor":20,"mc_list":[{"id":7000000000000000001,"id_str":"7000000000000000001","title":"s","author":"x","duration":30}]}`})
	if p.Class != ResponseOK || len(p.Musics) != 1 || p.Musics[0].DurationMS != 30000 {
		t.Fatalf("unexpected music page: %+v", p)
	}
	if got := ParseProfile(PageReply{Status: 200, Body: `{"status_code":8}`}).Class; got != ResponseAuthRequired {
		t.Fatalf("self code 8 must require login, got %s", got)
	}
	pr := ParseProfile(PageReply{Status: 200, Body: `{"status_code":0,"user":{"sec_uid":"MS4wLjABAAAA","favoriting_count":12}}`})
	if pr.Class != ResponseOK || pr.Likes != 12 {
		t.Fatalf("unexpected profile: %+v", pr)
	}
}

func TestParseListRecommend(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantClass  ResponseClass
		wantCount  int
		wantMore   bool
		wantCursor string
	}{
		{
			name:       "normal page with max_cursor",
			body:       `{"status_code":0,"has_more":1,"max_cursor":1700000000000,"cursor":999,"aweme_list":[{"aweme_id":"7300000000000000003","desc":"rec","create_time":1700000000,"author":{"nickname":"a","sec_uid":"MS4wLjABAAAA"},"video":{"duration":12000,"cover":{"url_list":["https://p3.douyinpic.com/r.jpg"]}}}]}`,
			wantClass:  ResponseOK,
			wantCount:  1,
			wantMore:   true,
			wantCursor: "1700000000000",
		},
		{
			name:       "fallback to cursor when max_cursor empty",
			body:       `{"status_code":0,"has_more":1,"cursor":"1700000000123","aweme_list":[{"aweme_id":"7300000000000000004","desc":"rec2"}]}`,
			wantClass:  ResponseOK,
			wantCount:  1,
			wantMore:   true,
			wantCursor: "1700000000123",
		},
		{
			name:       "empty final page with null aweme_list",
			body:       `{"status_code":0,"has_more":0,"max_cursor":0,"aweme_list":null}`,
			wantClass:  ResponseOK,
			wantCount:  0,
			wantMore:   false,
			wantCursor: "0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := ParseList(OpRecommend, PageReply{Status: 200, Body: tc.body})
			if p.Class != tc.wantClass || len(p.Works) != tc.wantCount || p.HasMore != tc.wantMore || p.Cursor != tc.wantCursor {
				t.Fatalf("got %+v, want class=%s count=%d more=%t cursor=%s", p, tc.wantClass, tc.wantCount, tc.wantMore, tc.wantCursor)
			}
		})
	}
}

func TestListShapeDiagnostics(t *testing.T) {
	cases := []struct {
		name     string
		reply    PageReply
		contains []string
		forbid   []string
	}{
		{
			name:     "invalid json",
			reply:    PageReply{Status: 500, Body: "not-json"},
			contains: []string{"http=500", "bytes=8", "json=false", "keys=[]", "aweme_count=-1", "item_keys=[]"},
		},
		{
			name:     "null aweme_list and token",
			reply:    PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"max_cursor":123,"whale_cut_token":"secret-token-val","aweme_list":null}`},
			contains: []string{"http=200", "json=true", "keys=[aweme_list,has_more,max_cursor,status_code,whale_cut_token]", "has_more=true", "max_cursor=true", "cursor=false", "whale_cut_token=true", "aweme_count=-1", "item_keys=[]"},
			forbid:   []string{"secret-token-val", "123"},
		},
		{
			name:     "non-empty aweme_list",
			reply:    PageReply{Status: 200, Body: `{"status_code":0,"has_more":1,"cursor":"c1","aweme_list":[{"aweme_id":"7300000000000000009","desc":"private-title","author":{"nickname":"private-author"}}]}`},
			contains: []string{"http=200", "json=true", "keys=[aweme_list,cursor,has_more,status_code]", "has_more=true", "max_cursor=false", "cursor=true", "whale_cut_token=false", "aweme_count=1", "item_keys=[author,aweme_id,desc]"},
			forbid:   []string{"7300000000000000009", "private-title", "private-author", "c1"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ListShapeDiagnostics(tc.reply)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("got %q, missing %q", got, want)
				}
			}
			for _, secret := range tc.forbid {
				if strings.Contains(got, secret) {
					t.Errorf("got %q, leaked %q", got, secret)
				}
			}
		})
	}
}
