package douyin

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	hevc   = regexp.MustCompile(`(?i)bytevc1|h265|hevc`)
	gear4K = regexp.MustCompile(`(^|_)4(_|$)`)
)

func firstHTTPS(list []string) string {
	for _, u := range list {
		if strings.HasPrefix(u, "https://") {
			return u
		}
	}
	return ""
}

// DetailQuality describes one H.264 resolution tier returned by the detail endpoint.
type DetailQuality struct {
	ID      string
	Label   string
	Height  int
	BitRate int
	URLs    []string
}

// Detail is the playable metadata returned by the item detail endpoint.
type Detail struct {
	Class       ResponseClass
	Videos      []string
	Qualities   []DetailQuality
	Images      []DetailImage
	Music       DetailMusic
	Author      DetailAuthor
	Desc        string
	PublishedAt int64
	Liked       bool
	Favorited   bool
	// Sources describes the video candidates by kind and gear name; never URLs.
	Sources string
	// Problem names the decode error or the reason Douyin withheld the item; never values.
	Problem string
}

type DetailImage struct {
	URL     string
	LiveURL string
}
type DetailMusic struct{ Title, Author, URL string }
type DetailAuthor struct{ Nickname, Avatar string }

type videoURLList struct {
	urlList
	Width  json.Number `json:"width"`
	Height json.Number `json:"height"`
}

type rawDetail struct {
	AwemeID     json.Number     `json:"aweme_id"`
	Desc        string          `json:"desc"`
	CreateTime  json.Number     `json:"create_time"`
	UserDigged  json.RawMessage `json:"user_digged"`
	CollectStat json.RawMessage `json:"collect_stat"`
	Author      struct {
		Nickname    string   `json:"nickname"`
		AvatarThumb *urlList `json:"avatar_thumb"`
	} `json:"author"`
	Video *struct {
		PlayH264 *urlList `json:"play_addr_h264"`
		BitRate  []struct {
			GearName  string        `json:"gear_name"`
			BitRate   json.Number   `json:"bit_rate"`
			IsBytevc1 json.Number   `json:"is_bytevc1"`
			IsH265    json.Number   `json:"is_h265"`
			PlayAddr  *videoURLList `json:"play_addr"`
		} `json:"bit_rate"`
		PlayAddr     *urlList `json:"play_addr"`
		DownloadAddr *urlList `json:"download_addr"`
	} `json:"video"`
	Images []struct {
		URLList []string `json:"url_list"`
		Video   struct {
			PlayAddr *urlList `json:"play_addr"`
			URLList  []string `json:"url_list"`
		} `json:"video"`
		LiveImage *urlList `json:"live_image"`
	} `json:"images"`
	Music *struct {
		Title          string   `json:"title"`
		Author         string   `json:"author"`
		PlayURL        *urlList `json:"play_url"`
		URLList        []string `json:"url_list"`
		MatchedPGCSong *struct {
			PlayURL *urlList `json:"play_url"`
		} `json:"matched_pgc_sound"`
	} `json:"music"`
}

// ParseDetail parses an item detail reply and preserves candidate order.
func ParseDetail(reply PageReply) Detail {
	obj, class, _ := classify(OpDetail, reply)
	out := Detail{Class: class}
	if class != ResponseOK {
		return out
	}
	body := obj["aweme_detail"]
	if len(body) == 0 || string(body) == "null" {
		// Deleted, private or region-locked items come back without a detail.
		out.Class, out.Problem = ResponseAPIRejected, "WITHHELD"
		if raw, ok := obj["filter_detail"]; ok && len(raw) > 2 && string(raw) != "null" {
			var filter struct {
				Reason json.RawMessage `json:"filter_reason"`
				Msg    string          `json:"detail_msg"`
			}
			_ = json.Unmarshal(raw, &filter)
			out.Problem = withheldReason(filter.Msg)
		}
		return out
	}
	var raw rawDetail
	if err := json.Unmarshal(body, &raw); err != nil {
		return Detail{Class: ResponseAPIRejected, Problem: "DECODE " + err.Error()}
	}
	seen := map[string]bool{}
	add := func(url string) {
		if len(out.Videos) < 12 && strings.HasPrefix(url, "https://") && !seen[url] {
			seen[url] = true
			out.Videos = append(out.Videos, url)
		}
	}
	if raw.Video != nil {
		var parts []string
		parts = append(parts, "h264="+strconv.Itoa(len(raw.Video.PlayH264.all())))
		for _, br := range raw.Video.BitRate {
			parts = append(parts, br.GearName)
		}
		parts = append(parts, "play="+strconv.Itoa(len(raw.Video.PlayAddr.all())), "download="+strconv.Itoa(len(raw.Video.DownloadAddr.all())))
		out.Sources = strings.Join(parts, ",")
		for _, u := range raw.Video.PlayH264.all() {
			add(u)
		}
		type qualityCandidate struct {
			gearName string
			bitRate  int
			urls     []string
		}
		type qualityGroup struct {
			id         string
			label      string
			rank       int
			candidates []qualityCandidate
		}
		groups := map[string]*qualityGroup{}
		for _, br := range raw.Video.BitRate {
			// WebKit cannot rely on Douyin's H.265 streams; only H.264 gears are candidates.
			if hevc.MatchString(br.GearName) || num(br.IsBytevc1) == 1 || num(br.IsH265) == 1 {
				continue
			}
			urls := br.PlayAddr.all()
			for _, u := range urls {
				add(u)
			}
			if len(urls) == 0 {
				continue
			}
			var width, height int
			if br.PlayAddr != nil {
				width = int(num(br.PlayAddr.Width))
				height = int(num(br.PlayAddr.Height))
			}
			id, label, rank := parseQualityTier(br.GearName, width, height)
			if id == "" {
				continue
			}
			g := groups[id]
			if g == nil {
				g = &qualityGroup{id: id, label: label, rank: rank}
				groups[id] = g
			}
			g.candidates = append(g.candidates, qualityCandidate{
				gearName: br.GearName,
				bitRate:  int(num(br.BitRate)),
				urls:     urls,
			})
		}
		for _, g := range groups {
			sort.SliceStable(g.candidates, func(i, j int) bool {
				if g.candidates[i].bitRate != g.candidates[j].bitRate {
					return g.candidates[i].bitRate > g.candidates[j].bitRate
				}
				return gearPriority(g.candidates[i].gearName) > gearPriority(g.candidates[j].gearName)
			})
			tierSeen := map[string]bool{}
			var tierURLs []string
			for _, c := range g.candidates {
				for _, u := range c.urls {
					if strings.HasPrefix(u, "https://") && !tierSeen[u] {
						tierSeen[u] = true
						tierURLs = append(tierURLs, u)
					}
				}
			}
			if len(tierURLs) > 0 {
				out.Qualities = append(out.Qualities, DetailQuality{
					ID:      g.id,
					Label:   g.label,
					Height:  g.rank,
					BitRate: g.candidates[0].bitRate,
					URLs:    tierURLs,
				})
			}
		}
		sort.SliceStable(out.Qualities, func(i, j int) bool {
			return out.Qualities[i].Height > out.Qualities[j].Height
		})
		for _, u := range raw.Video.PlayAddr.all() {
			add(u)
		}
		// The download stream is H.264 with a watermark; it plays when nothing else does.
		for _, u := range raw.Video.DownloadAddr.all() {
			add(u)
		}
	}
	for _, image := range raw.Images {
		for _, u := range image.URLList {
			if strings.HasPrefix(u, "https://") {
				item := DetailImage{URL: u, LiveURL: image.Video.PlayAddr.first()}
				if item.LiveURL == "" {
					item.LiveURL = image.LiveImage.first()
				}
				if item.LiveURL == "" {
					item.LiveURL = firstHTTPS(image.Video.URLList)
				}
				out.Images = append(out.Images, item)
				break
			}
		}
	}
	if raw.Music != nil {
		out.Music = DetailMusic{Title: raw.Music.Title, Author: raw.Music.Author, URL: raw.Music.PlayURL.first()}
		if out.Music.URL == "" {
			out.Music.URL = firstHTTPS(raw.Music.URLList)
		}
		if out.Music.URL == "" && raw.Music.MatchedPGCSong != nil {
			out.Music.URL = raw.Music.MatchedPGCSong.PlayURL.first()
		}
	}
	out.Author = DetailAuthor{Nickname: raw.Author.Nickname, Avatar: raw.Author.AvatarThumb.first()}
	out.Desc, out.PublishedAt = raw.Desc, num(raw.CreateTime)
	out.Liked, _ = truthy(raw.UserDigged)
	var collect struct {
		CollectCount json.Number `json:"collect_count"`
	}
	if json.Unmarshal(raw.CollectStat, &collect) == nil {
		out.Favorited = num(collect.CollectCount) > 0
	} else {
		out.Favorited = num(json.Number(strings.Trim(string(raw.CollectStat), `"`))) > 0
	}
	return out
}

// MusicDetail is the playable metadata returned by the music detail endpoint.
type MusicDetail struct {
	Class ResponseClass
	Music Music
}

// ParseMusicDetail parses a music detail reply and extracts playable metadata.
func ParseMusicDetail(reply PageReply) MusicDetail {
	obj, class, _ := classify(OpMusicDetail, reply)
	if class != ResponseOK {
		return MusicDetail{Class: class}
	}
	body := obj["music_info"]
	if len(body) == 0 || string(body) == "null" {
		return MusicDetail{Class: ResponseAPIRejected}
	}
	var raw rawMusic
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return MusicDetail{Class: ResponseAPIRejected}
	}
	music := raw.music()
	if music.PlayURL == "" {
		return MusicDetail{Class: ResponseAPIRejected}
	}
	return MusicDetail{Class: ResponseOK, Music: music}
}

// withheldReason maps Douyin's message for a withheld item to a stable code.
func withheldReason(msg string) string {
	switch {
	case strings.Contains(msg, "删除"):
		return "DELETED"
	case strings.Contains(msg, "私密") || strings.Contains(msg, "仅作者") || strings.Contains(msg, "自己可见"):
		return "PRIVATE"
	case strings.Contains(msg, "审核"):
		return "REVIEWING"
	}
	return "WITHHELD"
}

func parseQualityTier(gearName string, width, height int) (id, label string, rank int) {
	lower := strings.ToLower(gearName)
	switch {
	case strings.Contains(lower, "2160") || strings.Contains(lower, "4k") || gear4K.MatchString(lower):
		return "2160", "4K", 2160
	case strings.Contains(lower, "1440") || strings.Contains(lower, "2k"):
		return "1440", "2K", 1440
	case strings.Contains(lower, "1080"):
		return "1080", "1080P", 1080
	case strings.Contains(lower, "720"):
		return "720", "720P", 720
	case strings.Contains(lower, "540"):
		return "540", "540P", 540
	case strings.Contains(lower, "480"):
		return "480", "480P", 480
	case strings.Contains(lower, "360"):
		return "360", "360P", 360
	}
	dim := 0
	switch {
	case width > 0 && height > 0:
		dim = min(width, height)
	case width > 0:
		dim = width
	case height > 0:
		dim = height
	}
	switch {
	case dim >= 2000:
		return "2160", "4K", 2160
	case dim >= 1350:
		return "1440", "2K", 1440
	case dim >= 1000:
		return "1080", "1080P", 1080
	case dim >= 680:
		return "720", "720P", 720
	case dim >= 500:
		return "540", "540P", 540
	case dim >= 420:
		return "480", "480P", 480
	case dim > 0:
		return "360", "360P", 360
	}
	return "", "", 0
}

func gearPriority(gearName string) int {
	lower := strings.ToLower(gearName)
	switch {
	case strings.HasPrefix(lower, "normal_"):
		return 2
	case strings.HasPrefix(lower, "low_") || strings.HasPrefix(lower, "lower_"):
		return 0
	default:
		return 1
	}
}
