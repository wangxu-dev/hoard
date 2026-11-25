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

func TestParseMusicDetail(t *testing.T) {
	cases := []struct {
		name      string
		reply     PageReply
		wantClass ResponseClass
		wantMusic *Music
	}{
		{
			name:      "normal music_info",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"music_info":{"id":7000000000000000001,"id_str":"7000000000000000001","title":"Demo Song","author":"Artist","duration":32,"cover_medium":{"url_list":["https://img.invalid/m.jpg"]},"cover_thumb":{"url_list":["https://img.invalid/t.jpg"]},"play_url":{"url_list":["https://m.invalid/song.mp3"]}}}`},
			wantClass: ResponseOK,
			wantMusic: &Music{
				ID:         "7000000000000000001",
				Title:      "Demo Song",
				Author:     "Artist",
				DurationMS: 32000,
				Cover:      "https://img.invalid/m.jpg",
				Covers:     []string{"https://img.invalid/m.jpg", "https://img.invalid/t.jpg"},
				PlayURL:    "https://m.invalid/song.mp3",
			},
		},
		{
			name:      "matched_pgc_sound and owner_nickname fallback",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"music_info":{"id":7000000000000000002,"title":"PGC Song","owner_nickname":"Owner","duration":18,"cover_thumb":{"url_list":["https://img.invalid/thumb.jpg"]},"play_url":{"url_list":[]},"matched_pgc_sound":{"play_url":{"url_list":["https://m.invalid/pgc.mp3"]}}}}`},
			wantClass: ResponseOK,
			wantMusic: &Music{
				ID:         "7000000000000000002",
				Title:      "PGC Song",
				Author:     "Owner",
				DurationMS: 18000,
				Cover:      "https://img.invalid/thumb.jpg",
				Covers:     []string{"https://img.invalid/thumb.jpg"},
				PlayURL:    "https://m.invalid/pgc.mp3",
			},
		},
		{
			name:      "missing music_info",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"other":1}`},
			wantClass: ResponseAPIRejected,
		},
		{
			name:      "null music_info",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"music_info":null}`},
			wantClass: ResponseAPIRejected,
		},
		{
			name:      "missing play_url",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"music_info":{"id_str":"7000000000000000003","title":"Silent","play_url":{"url_list":[]}}}`},
			wantClass: ResponseAPIRejected,
		},
		{
			name:      "auth required",
			reply:     PageReply{Status: 401, Body: ""},
			wantClass: ResponseAuthRequired,
		},
		{
			name:      "risk control",
			reply:     PageReply{Status: 200, Body: `{"status_code":2154}`},
			wantClass: ResponseRiskControl,
		},
		{
			name:      "verification required",
			reply:     PageReply{Status: 200, Body: `{"status_code":0,"verify_page":{"a":1}}`},
			wantClass: ResponseVerificationRequired,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseMusicDetail(tc.reply)
			if got.Class != tc.wantClass {
				t.Fatalf("class = %s, want %s", got.Class, tc.wantClass)
			}
			if tc.wantMusic == nil {
				return
			}
			m := got.Music
			if m.ID != tc.wantMusic.ID || m.Title != tc.wantMusic.Title || m.Author != tc.wantMusic.Author ||
				m.DurationMS != tc.wantMusic.DurationMS || m.Cover != tc.wantMusic.Cover || m.PlayURL != tc.wantMusic.PlayURL ||
				len(m.Covers) != len(tc.wantMusic.Covers) {
				t.Fatalf("music = %+v, want %+v", m, *tc.wantMusic)
			}
			for i := range m.Covers {
				if m.Covers[i] != tc.wantMusic.Covers[i] {
					t.Fatalf("covers[%d] = %q, want %q", i, m.Covers[i], tc.wantMusic.Covers[i])
				}
			}
		})
	}
}
