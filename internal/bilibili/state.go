package bilibili

import "encoding/json"

// VideoState is personal state, not public like/coin counters. Nil means unknown.
type VideoState struct {
	Liked     *bool
	Favorited *bool
	Coins     *int
}

func ParseLiked(body []byte) (*bool, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var value int
	if json.Unmarshal(data, &value) != nil || (value != 0 && value != 1) {
		return nil, ErrInvalid
	}
	on := value == 1
	return &on, nil
}
func ParseFavorited(body []byte) (*bool, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Value *bool `json:"favoured"`
	}
	if json.Unmarshal(data, &result) != nil || result.Value == nil {
		return nil, ErrInvalid
	}
	return result.Value, nil
}
func ParseCoins(body []byte) (*int, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Value *int `json:"multiply"`
	}
	if json.Unmarshal(data, &result) != nil || result.Value == nil || *result.Value < 0 || *result.Value > 2 {
		return nil, ErrInvalid
	}
	return result.Value, nil
}

func ParseWatchLater(body []byte) ([]Video, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Count *int `json:"count"`
		List  []struct {
			AID       json.Number `json:"aid"`
			BVID      string      `json:"bvid"`
			Title     string      `json:"title"`
			Cover     string      `json:"pic"`
			Duration  int         `json:"duration"`
			Published int64       `json:"pubdate"`
			Added     int64       `json:"add_at"`
			State     int         `json:"state"`
			Owner     struct {
				ID   json.Number `json:"mid"`
				Name string      `json:"name"`
				Face string      `json:"face"`
			} `json:"owner"`
		} `json:"list"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Count == nil || *raw.Count != len(raw.List) {
		return nil, ErrInvalid
	}
	out := make([]Video, 0, len(raw.List))
	seen := map[string]bool{}
	for _, item := range raw.List {
		if !videoID.MatchString(item.BVID) || seen[item.BVID] || item.Duration < 0 || item.Duration > 864000 {
			return nil, ErrInvalid
		}
		seen[item.BVID] = true
		author, _ := id(item.Owner.ID)
		out = append(out, Video{AID: string(item.AID), ID: item.BVID, Title: item.Title, Cover: image(item.Cover), AuthorID: author, Author: item.Owner.Name, Avatar: image(item.Owner.Face), DurationMS: item.Duration * 1000, PublishedAt: item.Published, FavoritedAt: item.Added, Available: item.State == 0})
	}
	return out, nil
}
