package douyin

import "testing"

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
	if got := ParseProfile(PageReply{Status: 200, Body: `{"status_code":8}`}).Class; got != ResponseNotReady {
		t.Fatalf("self code 8 must not be risk, got %s", got)
	}
	pr := ParseProfile(PageReply{Status: 200, Body: `{"status_code":0,"user":{"sec_uid":"MS4wLjABAAAA","favoriting_count":12}}`})
	if pr.Class != ResponseOK || pr.Likes != 12 {
		t.Fatalf("unexpected profile: %+v", pr)
	}
}
