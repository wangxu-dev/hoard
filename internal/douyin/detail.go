package douyin

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var hevc = regexp.MustCompile(`(?i)bytevc1|h265|hevc`)

func firstHTTPS(list []string) string {
	for _, u := range list {
		if strings.HasPrefix(u, "https://") {
			return u
		}
	}
	return ""
}

// Detail is the playable metadata returned by the item detail endpoint.
type Detail struct {
	Class       ResponseClass
	Videos      []string
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
			GearName  string      `json:"gear_name"`
			IsBytevc1 json.Number `json:"is_bytevc1"`
			IsH265    json.Number `json:"is_h265"`
			PlayAddr  *urlList    `json:"play_addr"`
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
		for _, br := range raw.Video.BitRate {
			// WebKit cannot rely on Douyin's H.265 streams; only H.264 gears are candidates.
			if hevc.MatchString(br.GearName) || num(br.IsBytevc1) == 1 || num(br.IsH265) == 1 {
				continue
			}
			for _, u := range br.PlayAddr.all() {
				add(u)
			}
		}
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
