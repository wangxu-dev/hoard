package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var ErrAuth = errors.New("AUTH_REQUIRED")
var ErrUnavailable = errors.New("UNAVAILABLE")
var ErrInvalid = errors.New("INVALID_RESPONSE")
var numericID = regexp.MustCompile(`^[0-9]{1,30}$`)
var videoID = regexp.MustCompile(`^BV[0-9A-Za-z]{10}$`)

type Folder struct {
	ID    string
	Title string
	Count int
}
type Video struct {
	ID          string
	Title       string
	Cover       string
	AuthorID    string
	Author      string
	Avatar      string
	DurationMS  int
	PublishedAt int64
	FavoritedAt int64
	Available   bool
}
type Page struct {
	Videos  []Video
	HasMore bool
}

func payload(body []byte) (json.RawMessage, error) {
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Code == nil {
		return nil, ErrInvalid
	}
	switch *envelope.Code {
	case 0:
	case -101:
		return nil, ErrAuth
	default:
		return nil, ErrUnavailable
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, ErrInvalid
	}
	return envelope.Data, nil
}
func id(n json.Number) (string, error) {
	if !numericID.MatchString(string(n)) || string(n) == "0" {
		return "", ErrInvalid
	}
	return string(n), nil
}
func image(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Port() != "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "hdslb.com" && !strings.HasSuffix(host, ".hdslb.com") {
		return ""
	}
	u.Scheme = "https"
	return u.String()
}
func ParseFolders(body []byte) ([]Folder, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Count *int `json:"count"`
		List  []struct {
			ID    json.Number `json:"id"`
			Title string      `json:"title"`
			Count int         `json:"media_count"`
		} `json:"list"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Count == nil || *raw.Count < 0 || len(raw.List) != *raw.Count {
		return nil, ErrInvalid
	}
	out := make([]Folder, 0, len(raw.List))
	seen := map[string]bool{}
	for _, item := range raw.List {
		key, err := id(item.ID)
		if err != nil || item.Count < 0 || seen[key] {
			return nil, ErrInvalid
		}
		seen[key] = true
		out = append(out, Folder{key, item.Title, item.Count})
	}
	return out, nil
}
func ParsePage(body []byte) (Page, error) {
	data, err := payload(body)
	if err != nil {
		return Page{}, err
	}
	var raw struct {
		HasMore *bool `json:"has_more"`
		Medias  []struct {
			ID          json.Number `json:"id"`
			BVID        string      `json:"bvid"`
			Type        int         `json:"type"`
			Title       string      `json:"title"`
			Cover       string      `json:"cover"`
			Duration    int         `json:"duration"`
			PublishedAt int64       `json:"pubtime"`
			FavoritedAt int64       `json:"fav_time"`
			Attr        int         `json:"attr"`
			Upper       struct {
				ID   json.Number `json:"mid"`
				Name string      `json:"name"`
				Face string      `json:"face"`
			} `json:"upper"`
		} `json:"medias"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.HasMore == nil || (*raw.HasMore && len(raw.Medias) == 0) {
		return Page{}, ErrInvalid
	}
	out := Page{Videos: []Video{}, HasMore: *raw.HasMore}
	for _, item := range raw.Medias {
		if item.Type != 2 {
			continue
		}
		if !videoID.MatchString(item.BVID) || item.Duration < 0 || item.Duration > 864000 {
			return Page{}, fmt.Errorf("video metadata: %w", ErrInvalid)
		}
		author, err := id(item.Upper.ID)
		if err != nil {
			author = ""
		}
		out.Videos = append(out.Videos, Video{ID: item.BVID, Title: item.Title, Cover: image(item.Cover), AuthorID: author, Author: item.Upper.Name, Avatar: image(item.Upper.Face), DurationMS: item.Duration * 1000, PublishedAt: item.PublishedAt, FavoritedAt: item.FavoritedAt, Available: item.Attr&1 == 0})
	}
	return out, nil
}
