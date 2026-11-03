package bilibili

import (
	"encoding/json"
	"errors"
)

var ErrPaid = errors.New("PAID_CONTENT")

// PaidVideo identifies platform payment flags, independently of playback access.
func PaidVideo(body []byte) bool {
	data, err := payload(body)
	if err != nil {
		return false
	}
	var result struct {
		Exclusive bool `json:"is_upower_exclusive"`
		Rights    struct {
			Pay    int `json:"pay"`
			UGCPay int `json:"ugc_pay"`
		} `json:"rights"`
	}
	return json.Unmarshal(data, &result) == nil && (result.Exclusive || result.Rights.Pay == 1 || result.Rights.UGCPay == 1)
}

type Playback struct {
	PreviewMS  int64
	DurationMS int64
	URLs       []string
	Quality    int
	Qualities  []int
}

type VideoPart struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	DurationMS int64  `json:"durationMs"`
}

func ParseParts(body []byte) ([]VideoPart, error) {
	data, err := payload(body)
	if err != nil {
		return nil, err
	}
	var result struct {
		Pages []struct {
			CID      json.Number `json:"cid"`
			Title    string      `json:"part"`
			Duration int64       `json:"duration"`
		} `json:"pages"`
	}
	if json.Unmarshal(data, &result) != nil {
		return nil, ErrInvalid
	}
	parts := make([]VideoPart, 0, len(result.Pages))
	for _, p := range result.Pages {
		cid, err := id(p.CID)
		if err != nil || p.Duration < 0 {
			return nil, ErrInvalid
		}
		parts = append(parts, VideoPart{ID: cid, Title: p.Title, DurationMS: p.Duration * 1000})
	}
	return parts, nil
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
		Quality    int             `json:"quality"`
		Preview    json.RawMessage `json:"is_preview"`
		TimeLength int64           `json:"timelength"`
		Format     string          `json:"format"`
		Accept     []int           `json:"accept_quality"`
		URLs       []struct {
			Length int64    `json:"length"`
			URL    string   `json:"url"`
			Backup []string `json:"backup_url"`
		} `json:"durl"`
	}
	if json.Unmarshal(data, &result) != nil {
		return Playback{}, ErrInvalid
	}
	preview := string(result.Preview) == "1" || string(result.Preview) == "true"
	if len(result.URLs) != 1 || result.Format != "mp4" {
		if preview {
			return Playback{}, ErrPaid
		}
		return Playback{}, ErrUnavailable
	}
	out := Playback{Quality: result.Quality, Qualities: result.Accept, URLs: append([]string{result.URLs[0].URL}, result.URLs[0].Backup...)}
	if result.TimeLength > 0 {
		out.DurationMS = result.TimeLength
	}
	if out.DurationMS <= 0 && result.URLs[0].Length > 0 {
		out.DurationMS = result.URLs[0].Length
	}
	if preview {
		out.PreviewMS = out.DurationMS
	}
	return out, nil
}
