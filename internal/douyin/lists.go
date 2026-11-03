package douyin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Work is one video or gallery from a list endpoint.
type Work struct {
	ID          string
	Kind        string // video | gallery
	Title       string
	AuthorID    string
	Author      string
	AuthorIcon  string
	PublishedAt int64
	DurationMS  int
	Width       int
	Height      int
	Cover       string
	Covers      []string
	PlayURL     string
	Images      []string
	AwemeType   int
	IsLive      bool
}

// Music is one saved sound.
type Music struct {
	ID         string
	Title      string
	Author     string
	DurationMS int
	Cover      string
	Covers     []string
	PlayURL    string
}

// ListPage is one parsed page of a list endpoint.
type ListPage struct {
	Class    ResponseClass
	Code     int64
	Works    []Work
	Musics   []Music
	Received int
	Cursor   string
	HasMore  bool
}

// Profile is the subset of profile/self the sync needs.
type Profile struct {
	Class     ResponseClass
	SecUID    string
	Nickname  string
	Likes     int
	Favorites int
}

type urlList struct {
	URLList []string `json:"url_list"`
}

// all returns every https URL of the list, in order.
func (u *urlList) all() []string {
	if u == nil {
		return nil
	}
	var out []string
	for _, v := range u.URLList {
		if strings.HasPrefix(v, "https://") {
			out = append(out, v)
		}
	}
	return out
}

func (u *urlList) first() string {
	if u == nil {
		return ""
	}
	for _, v := range u.URLList {
		if strings.HasPrefix(v, "https://") {
			return v
		}
	}
	return ""
}

type rawAweme struct {
	AwemeID    json.Number `json:"aweme_id"`
	Desc       string      `json:"desc"`
	CreateTime json.Number `json:"create_time"`
	AwemeType  json.Number `json:"aweme_type"`
	Author     struct {
		Nickname    string   `json:"nickname"`
		SecUID      string   `json:"sec_uid"`
		UID         string   `json:"uid"`
		AvatarThumb *urlList `json:"avatar_thumb"`
	} `json:"author"`
	Video *struct {
		Duration    json.Number `json:"duration"`
		Width       json.Number `json:"width"`
		Height      json.Number `json:"height"`
		Cover       *urlList    `json:"cover"`
		OriginCover *urlList    `json:"origin_cover"`
		PlayAddr    *urlList    `json:"play_addr"`
		PlayH264    *urlList    `json:"play_addr_h264"`
	} `json:"video"`
	Images []struct {
		URLList   []string        `json:"url_list"`
		Video     json.RawMessage `json:"video"`
		LiveImage json.RawMessage `json:"live_image"`
	} `json:"images"`
}

type rawMusic struct {
	ID          json.Number `json:"id"`
	IDStr       string      `json:"id_str"`
	Title       string      `json:"title"`
	Author      string      `json:"author"`
	OwnerName   string      `json:"owner_nickname"`
	Duration    json.Number `json:"duration"`
	CoverMedium *urlList    `json:"cover_medium"`
	CoverThumb  *urlList    `json:"cover_thumb"`
	PlayURL     *urlList    `json:"play_url"`
}

func num(n json.Number) int64 {
	if v, err := n.Int64(); err == nil {
		return v
	}
	if f, err := n.Float64(); err == nil {
		return int64(f)
	}
	return 0
}

func cursorOf(raw json.RawMessage) string {
	s := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	if s == "null" {
		return ""
	}
	return s
}

func truthy(raw json.RawMessage) (value, ok bool) {
	switch strings.Trim(strings.TrimSpace(string(raw)), `"`) {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	}
	return false, false
}

var envelopeKeys = map[string]bool{"status_code": true, "status_msg": true, "extra": true, "log_pb": true, "logid": true, "now": true}

func decodeObject(body string) (map[string]json.RawMessage, bool) {
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.UseNumber()
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, false
	}
	return obj, true
}

func statusOf(obj map[string]json.RawMessage) (int64, string) {
	var code json.Number
	var msg string
	_ = json.Unmarshal(obj["status_code"], &code)
	_ = json.Unmarshal(obj["status_msg"], &msg)
	return num(code), msg
}

func envelopeOnly(obj map[string]json.RawMessage) bool {
	for k := range obj {
		if !envelopeKeys[k] {
			return false
		}
	}
	return true
}

// classify maps a raw reply to a response class before parsing.
func classify(op Operation, reply PageReply) (map[string]json.RawMessage, ResponseClass, int64) {
	if reply.Error != "" {
		return nil, ResponseNotReady, 0
	}
	body := strings.TrimSpace(reply.Body)
	if reply.Status == 401 || reply.Status == 403 || reply.Status == 429 || strings.HasPrefix(body, "<") {
		return nil, classifyResponse(string(op), reply.Status, 0, "", body, false), 0
	}
	if reply.Status < 200 || reply.Status >= 300 {
		return nil, ResponseNotReady, 0
	}
	obj, ok := decodeObject(body)
	if !ok || len(obj) == 0 {
		if op == OpSelf {
			return nil, ResponseNotReady, 0
		}
		// An empty reply on a list endpoint means the data was withheld.
		return nil, ResponseRiskControl, 0
	}
	code, msg := statusOf(obj)
	for _, key := range []string{"captcha", "verify_page", "verify_center_decision_conf"} {
		if raw, ok := obj[key]; ok && len(raw) > 0 && string(raw) != "null" && string(raw) != `""` && string(raw) != "{}" {
			return obj, ResponseVerificationRequired, code
		}
	}
	return obj, classifyResponse(string(op), reply.Status, code, msg, "", code == 0 && envelopeOnly(obj)), code
}

// ProfileDiagnostics records response shape without identity or response values.
func ProfileDiagnostics(reply PageReply) string {
	obj, valid := decodeObject(reply.Body)
	code, _ := statusOf(obj)
	var user map[string]json.RawMessage
	_ = json.Unmarshal(obj["user"], &user)
	var id string
	_ = json.Unmarshal(user["sec_uid"], &id)
	return fmt.Sprintf("http=%d bytes=%d json=%t status_present=%t code=%d envelope=%t user=%t user_id=%t", reply.Status, len(reply.Body), valid, len(obj["status_code"]) > 0, code, valid && envelopeOnly(obj), user != nil, id != "")
}

// ParseProfile reads profile/self.
func ParseProfile(reply PageReply) Profile {
	obj, class, _ := classify(OpSelf, reply)
	if class != ResponseOK {
		return Profile{Class: class}
	}
	var user struct {
		SecUID          string      `json:"sec_uid"`
		Nickname        string      `json:"nickname"`
		FavoritingCount json.Number `json:"favoriting_count"`
		CollectCount    json.Number `json:"collect_count"`
	}
	if err := json.Unmarshal(obj["user"], &user); err != nil || user.SecUID == "" {
		return Profile{Class: ResponseAuthRequired}
	}
	return Profile{Class: ResponseOK, SecUID: user.SecUID, Nickname: user.Nickname, Likes: int(num(user.FavoritingCount)), Favorites: int(num(user.CollectCount))}
}

// ParseList reads one page of likes, favorites or music.
func ParseList(op Operation, reply PageReply) ListPage {
	obj, class, code := classify(op, reply)
	page := ListPage{Class: class, Code: code}
	if class != ResponseOK {
		return page
	}
	more, ok := truthy(obj["has_more"])
	if !ok {
		page.Class = ResponseAPIRejected
		return page
	}
	page.HasMore = more
	if op == OpLikes {
		page.Cursor = cursorOf(obj["max_cursor"])
	} else {
		page.Cursor = cursorOf(obj["cursor"])
		if page.Cursor == "" {
			page.Cursor = cursorOf(obj["max_cursor"])
		}
	}
	field := "aweme_list"
	if op == OpMusic {
		field = "mc_list"
	}
	raw, present := obj[field]
	if !present || string(raw) == "null" {
		if op != OpLikes && !more {
			return page
		}
		page.Class = ResponseAPIRejected
		return page
	}
	if op == OpMusic {
		var list []rawMusic
		if err := json.Unmarshal(raw, &list); err != nil {
			page.Class = ResponseAPIRejected
			return page
		}
		page.Received = len(list)
		for _, m := range list {
			id := m.IDStr
			if id == "" {
				id = m.ID.String()
			}
			if id == "" || id == "0" {
				continue
			}
			author := m.Author
			if author == "" {
				author = m.OwnerName
			}
			cover := m.CoverMedium.first()
			if cover == "" {
				cover = m.CoverThumb.first()
			}
			covers := append(m.CoverMedium.all(), m.CoverThumb.all()...)
			page.Musics = append(page.Musics, Music{ID: id, Title: m.Title, Author: author, DurationMS: int(num(m.Duration)) * 1000, Cover: cover, Covers: covers, PlayURL: m.PlayURL.first()})
		}
		return page
	}
	var list []rawAweme
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&list); err != nil {
		page.Class = ResponseAPIRejected
		return page
	}
	page.Received = len(list)
	for _, item := range list {
		id := item.AwemeID.String()
		if id == "" || id == "0" {
			continue
		}
		w := Work{ID: id, Kind: "video", Title: item.Desc, Author: item.Author.Nickname, AuthorID: item.Author.SecUID,
			AuthorIcon: item.Author.AvatarThumb.first(), PublishedAt: num(item.CreateTime), AwemeType: int(num(item.AwemeType))}
		if w.AuthorID == "" {
			w.AuthorID = item.Author.UID
		}
		if v := item.Video; v != nil {
			w.DurationMS = int(num(v.Duration))
			w.Width, w.Height = int(num(v.Width)), int(num(v.Height))
			w.Cover = v.Cover.first()
			if w.Cover == "" {
				w.Cover = v.OriginCover.first()
			}
			w.Covers = append(v.Cover.all(), v.OriginCover.all()...)
			w.PlayURL = v.PlayH264.first()
			if w.PlayURL == "" {
				w.PlayURL = v.PlayAddr.first()
			}
		}
		if len(item.Images) > 0 {
			w.Kind = "gallery"
			for _, img := range item.Images {
				u := (&urlList{URLList: img.URLList}).first()
				if u != "" {
					w.Images = append(w.Images, u)
				}
				if (len(img.Video) > 0 && string(img.Video) != "null") || (len(img.LiveImage) > 0 && string(img.LiveImage) != "null") {
					w.IsLive = true
				}
			}
			if w.Cover == "" && len(w.Images) > 0 {
				w.Cover = w.Images[0]
			}
		}
		page.Works = append(page.Works, w)
	}
	return page
}
