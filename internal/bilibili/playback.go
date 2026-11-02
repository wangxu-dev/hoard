package bilibili

import "encoding/json"

type Playback struct {
	URLs      []string
	Quality   int
	Qualities []int
}

func ParseCID(body []byte) (string, error) {
	data, err := payload(body)
	if err != nil {
		return "", err
	}
	var result struct {
		CID json.Number `json:"cid"`
	}
	if json.Unmarshal(data, &result) != nil {
		return "", ErrInvalid
	}
	return id(result.CID)
}

// ParsePlayback accepts a complete progressive file; segmented responses cannot
// be treated as interchangeable fallback URLs.
func ParsePlayback(body []byte) (Playback, error) {
	data, err := payload(body)
	if err != nil {
		return Playback{}, err
	}
	var result struct {
		Quality int    `json:"quality"`
		Format  string `json:"format"`
		Accept  []int  `json:"accept_quality"`
		URLs    []struct {
			URL    string   `json:"url"`
			Backup []string `json:"backup_url"`
		} `json:"durl"`
	}
	if json.Unmarshal(data, &result) != nil {
		return Playback{}, ErrInvalid
	}
	if len(result.URLs) != 1 || result.Format != "mp4" {
		return Playback{}, ErrUnavailable
	}
	out := Playback{Quality: result.Quality, Qualities: result.Accept, URLs: append([]string{result.URLs[0].URL}, result.URLs[0].Backup...)}
	return out, nil
}
