package douyin

import "testing"

func TestParseDetailVideoCandidates(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"desc":"demo","create_time":1700000000,"user_digged":1,"author":{"nickname":"Author","avatar_thumb":{"url_list":["https://img.invalid/a"]}},"video":{"play_addr_h264":{"url_list":["https://v.invalid/a","https://v.invalid/a"]},"bit_rate":[{"play_addr":{"url_list":["https://v.invalid/b"]}}],"play_addr":{"url_list":["https://v.invalid/c"]}}}}`})
	if p.Class != ResponseOK || len(p.Videos) != 3 || !p.Liked || p.Author.Nickname != "Author" {
		t.Fatalf("unexpected detail: %+v", p)
	}
}

func TestParseDetailGalleryLiveAndMusic(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"images":[{"url_list":["https://img.invalid/1"],"video":{"play_addr":{"url_list":["https://v.invalid/live"]}}},{"url_list":["https://img.invalid/2"]}],"music":{"title":"Song","author":"Singer","play_url":{"url_list":["https://m.invalid/a"]}},"collect_stat":{"collect_count":1}}}`})
	if p.Class != ResponseOK || len(p.Images) != 2 || p.Images[0].LiveURL == "" || p.Music.URL == "" || !p.Favorited {
		t.Fatalf("unexpected gallery: %+v", p)
	}
}

func TestParseDetailMissingAndEmptyURLs(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"video":{"play_addr_h264":{"url_list":[]}},"images":[{"url_list":[]}]}}`})
	if p.Class != ResponseOK || len(p.Videos) != 0 || len(p.Images) != 0 {
		t.Fatalf("unexpected empty detail: %+v", p)
	}
}

func TestParseDetailToleratesScalarCollectStat(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"aweme_id":"1","collect_stat":1,"video":{"play_addr_h264":{"url_list":["https://v.invalid/h264"]}}}}`})
	if p.Class != ResponseOK || len(p.Videos) != 1 || !p.Favorited {
		t.Fatalf("detail = %+v", p)
	}
}

func TestParseDetailNamesWithheldItems(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":null,"filter_detail":{"filter_reason":"status_deleted","detail_msg":"作品已删除"}}`})
	if p.Problem != "DELETED" {
		t.Fatalf("problem = %q", p.Problem)
	}
	p = ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":null,"filter_detail":{"detail_msg":"因作者设置，仅作者自己可见"}}`})
	if p.Problem != "PRIVATE" {
		t.Fatalf("problem = %q", p.Problem)
	}
}

func TestParseDetailSkipsHEVCGears(t *testing.T) {
	p := ParseDetail(PageReply{Status: 200, Body: `{"status_code":0,"aweme_detail":{"video":{"bit_rate":[{"gear_name":"normal_1080_bytevc1","play_addr":{"url_list":["https://v.invalid/hevc"]}},{"gear_name":"normal_720_0","play_addr":{"url_list":["https://v.invalid/avc"]}}]}}}`})
	if len(p.Videos) != 1 || p.Videos[0] != "https://v.invalid/avc" {
		t.Fatalf("videos = %v", p.Videos)
	}
}
