package app

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/store"
)

func playbackError(err error) error {
	if errors.Is(err, bilibili.ErrAuth) || errors.Is(err, bilibili.ErrPaid) {
		return err
	}
	return errors.New("UNAVAILABLE")
}

func (p *Player) bilibiliMedia(ctx context.Context, rec store.MediaRecord, requestedPart ...string) (MediaResult, error) {
	engine, ok := p.app.Engine.(interface {
		ReadBilibili(context.Context, string, map[string]string) ([]byte, error)
		Source() string
	})
	if !ok || engine.Source() != "bilibili" {
		log.Printf("[hoard:player] Bilibili session unavailable supported=%t source_match=%t", ok, ok && engine.Source() == "bilibili")
		return MediaResult{}, errors.New("SESSION_NOT_READY")
	}
	p.app.mu.Lock()
	session, accountID := p.app.sessionCtx, p.app.accountID
	p.app.mu.Unlock()
	if session == nil {
		return MediaResult{}, errors.New("SESSION_NOT_READY")
	}
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(session, cancel)
	defer stop()
	p.app.frontRequests.Add(1)
	defer p.app.frontRequests.Add(-1)
	read := func(path string, params map[string]string) ([]byte, error) {
		p.app.requestMu.Lock()
		defer p.app.requestMu.Unlock()
		if delay := time.Until(p.app.lastRequest.Add(pageGap)); delay > 0 && !wait(call, delay) {
			return nil, errors.New("CANCELED")
		}
		if call.Err() != nil {
			return nil, errors.New("CANCELED")
		}
		for {
			body, err := engine.ReadBilibili(call, path, params)
			p.app.lastRequest = time.Now()
			if err == nil || err.Error() != "NOT_READY" {
				return body, err
			}
			if !wait(call, 300*time.Millisecond) {
				return nil, errors.New("CANCELED")
			}
		}
	}
	if strings.HasPrefix(rec.RemoteID, "opus:") {
		body, err := read("/x/polymer/web-dynamic/v1/opus/detail", map[string]string{"id": strings.TrimPrefix(rec.RemoteID, "opus:"), "features": "htmlNewStyle"})
		if err != nil {
			return MediaResult{}, errors.New("REQUEST_FAILED")
		}
		opus, err := bilibili.ParseOpus(body)
		if err != nil {
			return MediaResult{}, playbackError(err)
		}
		if opus.ID != strings.TrimPrefix(rec.RemoteID, "opus:") {
			return MediaResult{}, errors.New("UNAVAILABLE")
		}
		out := MediaResult{Kind: "gallery", Author: rec.Author, Desc: opus.Text, PublishedAt: opus.PublishedAt, Complete: true}
		if opus.Author != nil {
			out.Author = opus.Author.Nickname
			out.AuthorAvatar = p.app.Media.Register(opus.Author.AvatarURL)
		}
		for _, raw := range opus.Images {
			if u := p.app.Media.Register(raw); u != "" {
				out.Images = append(out.Images, MediaImage{URL: u})
			}
		}
		return out, nil
	}
	body, err := read("/x/web-interface/view", map[string]string{"bvid": rec.RemoteID})
	if err != nil {
		return MediaResult{}, errors.New("REQUEST_FAILED")
	}
	paid := bilibili.PaidVideo(body)
	cid, err := bilibili.ParseCID(body)
	if err != nil {
		return MediaResult{}, playbackError(err)
	}
	parts, err := bilibili.ParseParts(body)
	if err != nil {
		return MediaResult{}, playbackError(err)
	}
	if len(requestedPart) > 0 && requestedPart[0] != "" {
		found := false
		for _, part := range parts {
			if part.ID == requestedPart[0] {
				cid = part.ID
				found = true
				break
			}
		}
		if !found {
			return MediaResult{}, errors.New("INVALID_ARGUMENT")
		}
	}
	body, err = read("/x/player/playurl", map[string]string{"bvid": rec.RemoteID, "cid": cid, "qn": strconv.Itoa(16), "fnval": "1", "fnver": "0", "fourk": "0", "platform": "html5"})
	if err != nil {
		return MediaResult{}, errors.New("REQUEST_FAILED")
	}
	playback, err := bilibili.ParsePlayback(body)
	if err != nil {
		var shape struct {
			Code int `json:"code"`
			Data struct {
				Format  string            `json:"format"`
				Quality int               `json:"quality"`
				DURL    []json.RawMessage `json:"durl"`
				DASH    json.RawMessage   `json:"dash"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &shape)
		log.Printf("[hoard:player] Bilibili stream rejected code=%d quality=%d format=%s segments=%d dash=%t", shape.Code, shape.Data.Quality, shape.Data.Format, len(shape.Data.DURL), len(shape.Data.DASH) > 0 && string(shape.Data.DASH) != "null")
		return MediaResult{}, playbackError(err)
	}
	out := MediaResult{Kind: "video", PreviewMS: playback.PreviewMS, Parts: parts, PartID: cid, Author: rec.Author, Desc: rec.Title, PublishedAt: rec.PublishedAt, Complete: true}
	if paid && out.PreviewMS == 0 {
		for _, part := range parts {
			if part.ID == cid && playback.DurationMS > 0 && playback.DurationMS+2000 < part.DurationMS {
				out.PreviewMS = playback.DurationMS
				break
			}
		}
	}
	for _, raw := range playback.URLs {
		if url := p.app.Media.Register(raw); url != "" {
			out.Videos = append(out.Videos, url)
		}
	}
	if len(out.Videos) == 0 {
		return MediaResult{}, errors.New("UNAVAILABLE")
	}
	if call.Err() != nil {
		return MediaResult{}, errors.New("CANCELED")
	}
	out.ExpiresAt = time.Now().Add(20 * time.Minute).UnixMilli()
	var archive struct {
		AID string `json:"aid"`
	}
	if json.Unmarshal(rec.Extra, &archive) == nil && archive.AID != "" {
		go func() {
			ctx, cancel := context.WithTimeout(session, 30*time.Second)
			defer cancel()
			client := bilibili.Client{Persist: p.app.bilibiliPersistence(), Read: func(ctx context.Context, path string, params map[string]string) ([]byte, error) {
				for {
					p.app.requestMu.Lock()
					if p.app.frontRequests.Load() == 0 {
						break
					}
					p.app.requestMu.Unlock()
					if !wait(ctx, 20*time.Millisecond) {
						return nil, ctx.Err()
					}
				}
				defer p.app.requestMu.Unlock()
				if delay := time.Until(p.app.lastRequest.Add(pageGap)); delay > 0 && !wait(ctx, delay) {
					return nil, ctx.Err()
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				body, err := engine.ReadBilibili(ctx, path, params)
				p.app.lastRequest = time.Now()
				return body, err
			}}
			if err := client.Observe(ctx, p.app.Store, accountID, bilibili.Video{ID: rec.RemoteID, AID: archive.AID}); err != nil && ctx.Err() == nil {
				log.Print("[hoard:player] Bilibili personal state refresh failed")
			}
		}()
	}
	return out, nil
}
