package bilibili

import (
	"encoding/json"
	"strings"
)

type ActionFolder struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Count    int    `json:"count"`
	Selected bool   `json:"selected"`
}

func ValidWorkID(value string) bool {
	return videoID.MatchString(value) || (strings.HasPrefix(value, "opus:") && numericID.MatchString(strings.TrimPrefix(value, "opus:")) && strings.TrimPrefix(value, "opus:") != "0")
}

func ParseActionFolders(body []byte) ([]ActionFolder, error) {
	folders, err := ParseFolders(body)
	if err != nil {
		return nil, err
	}
	data, _ := payload(body)
	var raw struct {
		List []struct {
			Selected *int `json:"fav_state"`
		} `json:"list"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil, ErrInvalid
	}
	out := make([]ActionFolder, len(folders))
	for i, f := range folders {
		selected := raw.List[i].Selected
		if selected == nil || (*selected != 0 && *selected != 1) {
			return nil, ErrInvalid
		}
		out[i] = ActionFolder{ID: f.ID, Title: f.Title, Count: f.Count, Selected: *selected == 1}
	}
	return out, nil
}

func ParseAID(body []byte, bvid string) (string, error) {
	data, err := payload(body)
	if err != nil {
		return "", err
	}
	var raw struct {
		AID  json.Number `json:"aid"`
		BVID string      `json:"bvid"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.BVID != bvid {
		return "", ErrInvalid
	}
	return id(raw.AID)
}

func ParseOpusState(body []byte, opusID string) (liked, favorited bool, err error) {
	data, err := payload(body)
	if err != nil {
		return false, false, err
	}
	var raw struct {
		Item struct {
			ID      string `json:"id_str"`
			Modules []struct {
				Stat *struct {
					Like struct {
						Status *bool `json:"status"`
					} `json:"like"`
					Favorite struct {
						Status *bool `json:"status"`
					} `json:"favorite"`
				} `json:"module_stat"`
			} `json:"modules"`
		} `json:"item"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Item.ID != opusID {
		return false, false, ErrInvalid
	}
	for _, m := range raw.Item.Modules {
		if m.Stat != nil && m.Stat.Like.Status != nil && m.Stat.Favorite.Status != nil {
			return *m.Stat.Like.Status, *m.Stat.Favorite.Status, nil
		}
	}
	return false, false, ErrInvalid
}
