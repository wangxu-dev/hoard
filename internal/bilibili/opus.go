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
	ID     string
	Text   string
	Images []string
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
		works = append(works, store.Work{RemoteID: "opus:" + v.ID, Kind: "gallery", Title: v.Content, CoverURL: image(v.Cover.URL)})
	}
	return works, *raw.HasMore, nil
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
				Content struct {
					Paragraphs []struct {
						Text struct {
							Nodes []struct {
								Word struct {
									Words string `json:"words"`
								} `json:"word"`
							} `json:"nodes"`
						} `json:"text"`
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
		add(m.Top.Display.Album.Pics)
		for _, p := range m.Content.Paragraphs {
			var line strings.Builder
			for _, n := range p.Text.Nodes {
				line.WriteString(n.Word.Words)
			}
			if line.Len() > 0 {
				out.Text += "\n" + line.String()
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
