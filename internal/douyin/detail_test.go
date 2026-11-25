package douyin

import (
	"reflect"
	"testing"
)

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
	if len(p.Qualities) != 1 || p.Qualities[0].ID != "720" || p.Qualities[0].Label != "720P" {
		t.Fatalf("qualities = %+v", p.Qualities)
	}
}

func TestParseQualityTier(t *testing.T) {
	cases := []struct {
		name     string
		gearName string
		width    int
		height   int
		wantID   string
		wantLbl  string
		wantRank int
	}{
		{name: "4k gear underscore", gearName: "adapt_lowest_4_1", wantID: "2160", wantLbl: "4K", wantRank: 2160},
		{name: "4k gear label", gearName: "normal_4k_0", wantID: "2160", wantLbl: "4K", wantRank: 2160},
		{name: "2160 gear", gearName: "normal_2160_0", wantID: "2160", wantLbl: "4K", wantRank: 2160},
		{name: "2k gear", gearName: "normal_2k_0", wantID: "1440", wantLbl: "2K", wantRank: 1440},
		{name: "1440 gear", gearName: "normal_1440_0", wantID: "1440", wantLbl: "2K", wantRank: 1440},
		{name: "1080 gear", gearName: "normal_1080_0", wantID: "1080", wantLbl: "1080P", wantRank: 1080},
		{name: "720 gear", gearName: "low_720_0", wantID: "720", wantLbl: "720P", wantRank: 720},
		{name: "540 gear", gearName: "normal_540_0", wantID: "540", wantLbl: "540P", wantRank: 540},
		{name: "480 gear", gearName: "normal_480_0", wantID: "480", wantLbl: "480P", wantRank: 480},
		{name: "360 gear", gearName: "lower_360_0", wantID: "360", wantLbl: "360P", wantRank: 360},
		{name: "fallback portrait 1080x1920", gearName: "custom", width: 1080, height: 1920, wantID: "1080", wantLbl: "1080P", wantRank: 1080},
		{name: "fallback 4k dims", gearName: "custom", width: 3840, height: 2160, wantID: "2160", wantLbl: "4K", wantRank: 2160},
		{name: "fallback 2k dims", gearName: "custom", width: 2560, height: 1440, wantID: "1440", wantLbl: "2K", wantRank: 1440},
		{name: "fallback 720 dims", gearName: "custom", width: 720, height: 1280, wantID: "720", wantLbl: "720P", wantRank: 720},
		{name: "fallback 540 dims", gearName: "custom", width: 540, height: 960, wantID: "540", wantLbl: "540P", wantRank: 540},
		{name: "fallback 480 dims", gearName: "custom", width: 480, height: 854, wantID: "480", wantLbl: "480P", wantRank: 480},
		{name: "fallback 360 dims", gearName: "custom", width: 360, height: 640, wantID: "360", wantLbl: "360P", wantRank: 360},
		{name: "fallback single dim", gearName: "", width: 720, height: 0, wantID: "720", wantLbl: "720P", wantRank: 720},
		{name: "unknown", gearName: "unknown", width: 0, height: 0, wantID: "", wantLbl: "", wantRank: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id, label, rank := parseQualityTier(tc.gearName, tc.width, tc.height)
			if id != tc.wantID || label != tc.wantLbl || rank != tc.wantRank {
				t.Fatalf("parseQualityTier(%q, %d, %d) = (%q, %q, %d), want (%q, %q, %d)",
					tc.gearName, tc.width, tc.height, id, label, rank, tc.wantID, tc.wantLbl, tc.wantRank)
			}
		})
	}
}

func TestParseDetailQualitiesGroupingSortingAndDedup(t *testing.T) {
	body := `{
		"status_code": 0,
		"aweme_detail": {
			"video": {
				"bit_rate": [
					{"gear_name": "normal_540_0", "bit_rate": 600000, "play_addr": {"width": 540, "height": 960, "url_list": ["https://v.invalid/540-a", "http://v.invalid/insecure"]}},
					{"gear_name": "low_720_0", "bit_rate": 900000, "play_addr": {"width": 720, "height": 1280, "url_list": ["https://v.invalid/720-low", "https://v.invalid/720-shared"]}},
					{"gear_name": "normal_1080_bytevc1", "is_bytevc1": 1, "bit_rate": 2500000, "play_addr": {"width": 1080, "height": 1920, "url_list": ["https://v.invalid/hevc-1080"]}},
					{"gear_name": "normal_1080_h265", "is_h265": 1, "bit_rate": 2400000, "play_addr": {"width": 1080, "height": 1920, "url_list": ["https://v.invalid/h265-1080"]}},
					{"gear_name": "normal_720_0", "bit_rate": 900000, "play_addr": {"width": 720, "height": 1280, "url_list": ["https://v.invalid/720-shared", "https://v.invalid/720-normal"]}},
					{"gear_name": "normal_1080_0", "bit_rate": 2200000, "play_addr": {"width": 1080, "height": 1920, "url_list": ["https://v.invalid/1080-a", "https://v.invalid/1080-a"]}},
					{"gear_name": "adapt_lowest_4_1", "bit_rate": 4500000, "play_addr": {"width": 2160, "height": 3840, "url_list": ["https://v.invalid/4k-a"]}}
				]
			}
		}
	}`
	p := ParseDetail(PageReply{Status: 200, Body: body})
	if p.Class != ResponseOK {
		t.Fatalf("class = %s", p.Class)
	}
	want := []DetailQuality{
		{ID: "2160", Label: "4K", Height: 2160, BitRate: 4500000, URLs: []string{"https://v.invalid/4k-a"}},
		{ID: "1080", Label: "1080P", Height: 1080, BitRate: 2200000, URLs: []string{"https://v.invalid/1080-a"}},
		{ID: "720", Label: "720P", Height: 720, BitRate: 900000, URLs: []string{"https://v.invalid/720-shared", "https://v.invalid/720-normal", "https://v.invalid/720-low"}},
		{ID: "540", Label: "540P", Height: 540, BitRate: 600000, URLs: []string{"https://v.invalid/540-a"}},
	}
	if !reflect.DeepEqual(p.Qualities, want) {
		t.Fatalf("qualities = %+v, want %+v", p.Qualities, want)
	}
	for _, u := range p.Videos {
		if u == "https://v.invalid/hevc-1080" || u == "https://v.invalid/h265-1080" {
			t.Fatalf("unexpected HEVC URL in Videos: %q", u)
		}
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
