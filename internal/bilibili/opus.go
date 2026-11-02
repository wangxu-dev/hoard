package bilibili

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/wangxu-dev/hoard/internal/store"
)

const opusFeed = "/x/polymer/web-dynamic/v1/opus/feed/fav"

type Opus struct {
	ID          string
	Text        string
	Images      []string
	Author      *store.Author
	PublishedAt int64
}

func ParseOpusPage(body []byte) ([]store.Work, bool, error) {
	data, err := payload(body)
	if err != nil {
		return nil, false, err
	}
	var raw struct {
		HasMore *bool `json:"has_more"`
		Items   []struct {
			ID      string `json:"opus_id"`
			Content string `json:"content"`
			AddedAt int64  `json:"fav_time"`
			Cover   struct {
				URL string `json:"url"`
			} `json:"cover"`
		} `json:"items"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.HasMore == nil || (*raw.HasMore && len(raw.Items) == 0) {
		return nil, false, ErrInvalid
	}
	works := make([]store.Work, 0, len(raw.Items))
	for _, v := range raw.Items {
		if !numericID.MatchString(v.ID) || v.ID == "0" {
			return nil, false, ErrInvalid
		}
		works = append(works, store.Work{AddedAt: v.AddedAt, RemoteID: "opus:" + v.ID, Kind: "gallery", Title: v.Content, CoverURL: image(v.Cover.URL)})
	}
	return works, *raw.HasMore, nil
}

type opusText struct {
	Nodes []struct {
		Word struct {
			Words string `json:"words"`
		} `json:"word"`
		Rich struct {
			Text     string `json:"text"`
			Original string `json:"orig_text"`
		} `json:"rich"`
		Formula struct {
			Text string `json:"latex_content"`
		} `json:"formula"`
	} `json:"nodes"`
}

func (t opusText) text() string {
	var out strings.Builder
	for _, n := range t.Nodes {
		out.WriteString(n.Word.Words)
		if n.Rich.Text != "" {
			out.WriteString(n.Rich.Text)
		} else {
			out.WriteString(n.Rich.Original)
		}
		out.WriteString(n.Formula.Text)
	}
	return out.String()
}

func ParseOpus(body []byte) (Opus, error) {
	data, err := payload(body)
	if err != nil {
		return Opus{}, err
	}
	type picture struct {
		URL string `json:"url"`
	}
	var raw struct {
		Item struct {
			ID    string `json:"id_str"`
			Basic struct {
				Title string `json:"title"`
			} `json:"basic"`
			Modules []struct {
				Author struct {
					ID          json.Number `json:"mid"`
					Name        string      `json:"name"`
					Face        string      `json:"face"`
					PublishedAt int64       `json:"pub_ts"`
				} `json:"module_author"`
				Content struct {
					Paragraphs []struct {
						Text opusText `json:"text"`
						List struct {
							Items []opusText `json:"items"`
						} `json:"list"`
						Code struct {
							Content string `json:"content"`
						} `json:"code"`
						Pic struct {
							Pics []picture `json:"pics"`
						} `json:"pic"`
					} `json:"paragraphs"`
				} `json:"module_content"`
				Top struct {
					Display struct {
						Album struct {
							Pics []picture `json:"pics"`
						} `json:"album"`
					} `json:"display"`
				} `json:"module_top"`
			} `json:"modules"`
		} `json:"item"`
	}
	if json.Unmarshal(data, &raw) != nil || !numericID.MatchString(raw.Item.ID) {
		return Opus{}, ErrInvalid
	}
	out := Opus{ID: raw.Item.ID, Text: raw.Item.Basic.Title}
	seen := map[string]bool{}
	add := func(pics []picture) {
		for _, p := range pics {
			if u := image(p.URL); u != "" && !seen[u] {
				seen[u] = true
				out.Images = append(out.Images, u)
			}
		}
	}
	for _, m := range raw.Item.Modules {
		if authorID, err := id(m.Author.ID); err == nil {
			out.Author = &store.Author{RemoteID: authorID, Nickname: m.Author.Name, AvatarURL: image(m.Author.Face)}
			out.PublishedAt = m.Author.PublishedAt
		}
		add(m.Top.Display.Album.Pics)
		for _, p := range m.Content.Paragraphs {
			if text := p.Text.text(); text != "" {
				out.Text += "\n" + text
			}
			for _, item := range p.List.Items {
				if text := item.text(); text != "" {
					out.Text += "\n" + text
				}
			}
			if p.Code.Content != "" {
				out.Text += "\n" + p.Code.Content
			}
			add(p.Pic.Pics)
		}
	}
	return out, nil
}

func (c Client) indexOpus(ctx context.Context, db *store.Store, accountID string) error {
	seen := map[[32]byte]bool{}
	for page := 1; page <= 10000; page++ {
		body, err := c.read(ctx, opusFeed, map[string]string{"page": strconv.Itoa(page), "page_size": "20"})
		if err != nil {
			return err
		}
		works, more, err := ParseOpusPage(body)
		if err != nil {
			return err
		}
		ids := make([]string, len(works))
		for i, w := range works {
			ids[i] = w.RemoteID
		}
		key := sha256.Sum256([]byte(strings.Join(ids, "\n")))
		if seen[key] {
			return fmt.Errorf("opus pagination stalled")
		}
		seen[key] = true
		for i := range works {
			body, err := c.read(ctx, "/x/polymer/web-dynamic/v1/opus/detail", map[string]string{"id": strings.TrimPrefix(works[i].RemoteID, "opus:"), "features": "htmlNewStyle"})
			if err != nil {
				return err
			}
			detail, err := ParseOpus(body)
			if err != nil {
				return err
			}
			if "opus:"+detail.ID != works[i].RemoteID {
				return ErrInvalid
			}
			works[i].Title, works[i].Author, works[i].PublishedAt = detail.Text, detail.Author, detail.PublishedAt
			works[i].ImageCount = len(detail.Images)
			if len(detail.Images) > 0 {
				works[i].CoverURL = detail.Images[0]
			}
		}
		if _, err = db.SaveWorks(ctx, accountID, "favorite", false, works); err != nil {
			return err
		}
		if c.Changed != nil {
			c.Changed("favorite")
		}
		if !more {
			return nil
		}
	}
	return fmt.Errorf("opus pagination limit exceeded")
}
