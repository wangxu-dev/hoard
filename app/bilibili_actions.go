package app

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/wangxu-dev/hoard/internal/bilibili"
	"github.com/wangxu-dev/hoard/internal/douyin"
)

type bilibiliActionEngine interface {
	ReadBilibili(context.Context, string, map[string]string) ([]byte, error)
	WriteBilibili(context.Context, string, map[string]string) ([]byte, error)
}

func (s *actionSession) read(ctx context.Context, kind string) (ActionState, error) {
	if s.account.Source == "douyin" {
		reply, err := s.douyinRequest(ctx, douyin.OpDetail, map[string]string{"aweme_id": s.work.RemoteID})
		if err != nil {
			return ActionState{}, errors.New("REQUEST_FAILED")
		}
		state, err := douyin.ParseActionState(reply, s.work.RemoteID)
		return ActionState{Liked: state.Liked, Favorited: state.Favorited, awemeType: state.AwemeType}, err
	}
	out := ActionState{}
	if strings.HasPrefix(s.work.RemoteID, "opus:") {
		if kind == "watch_later" {
			return out, errors.New("UNSUPPORTED_ACTION")
		}
		opusID := strings.TrimPrefix(s.work.RemoteID, "opus:")
		body, err := s.bilibiliRequest(ctx, false, "/x/polymer/web-dynamic/v1/opus/detail", map[string]string{"id": opusID, "features": "htmlNewStyle"})
		if err != nil {
			return out, err
		}
		out.Liked, out.Favorited, err = bilibili.ParseOpusState(body, opusID)
		return out, err
	}
	switch kind {
	case "like":
		body, err := s.bilibiliRequest(ctx, false, "/x/web-interface/archive/has/like", map[string]string{"bvid": s.work.RemoteID})
		if err != nil {
			return out, err
		}
		value, err := bilibili.ParseLiked(body)
		if err != nil {
			return out, err
		}
		out.Liked = *value
	case "favorite":
		aid, err := s.aid(ctx)
		if err != nil {
			return out, err
		}
		body, err := s.bilibiliRequest(ctx, false, "/x/v3/fav/folder/created/list-all", map[string]string{"up_mid": s.account.RemoteID, "type": "2", "rid": aid})
		if err != nil {
			return out, err
		}
		out.Folders, err = bilibili.ParseActionFolders(body)
		if err != nil {
			return out, err
		}
		for _, f := range out.Folders {
			out.Favorited = out.Favorited || f.Selected
		}
	case "watch_later":
		body, err := s.bilibiliRequest(ctx, false, "/x/v2/history/toview", nil)
		if err != nil {
			return out, err
		}
		videos, err := bilibili.ParseWatchLater(body)
		if err != nil {
			return out, err
		}
		for _, v := range videos {
			out.WatchLater = out.WatchLater || v.ID == s.work.RemoteID
		}
	}
	return out, nil
}

func (s *actionSession) write(ctx context.Context, kind string, on bool, folderIDs []string, before ActionState) error {
	if s.account.Source == "douyin" {
		value := "0"
		if on {
			value = "1"
		}
		op, params := douyin.OpSetLike, map[string]string{"aweme_id": s.work.RemoteID, "item_type": "0", "type": value}
		if kind == "favorite" {
			op, params = douyin.OpSetFavorite, map[string]string{"aweme_id": s.work.RemoteID, "action": value, "aweme_type": before.awemeType}
		}
		_, err := s.douyinRequest(ctx, op, params)
		return err
	}
	path, params := "", map[string]string{}
	value := "2"
	if on {
		value = "1"
	}
	if strings.HasPrefix(s.work.RemoteID, "opus:") {
		opusID := strings.TrimPrefix(s.work.RemoteID, "opus:")
		if folderIDs != nil {
			return bilibili.ErrInvalid
		}
		switch kind {
		case "like":
			path, params = "/dynamic_like/v1/dynamic_like/thumb", map[string]string{"dynamic_id": opusID, "up": value, "uid": s.account.RemoteID}
		case "favorite":
			action := "4"
			if on {
				action = "3"
			}
			path, params = "/x/community/cosmo/interface/simple_action", map[string]string{"id": opusID, "action": action}
		default:
			return bilibili.ErrInvalid
		}
	} else {
		switch kind {
		case "like":
			path, params = "/x/web-interface/archive/like", map[string]string{"bvid": s.work.RemoteID, "like": value}
		case "watch_later":
			aid, err := s.aid(ctx)
			if err != nil {
				return err
			}
			path = "/x/v2/history/toview/del"
			if on {
				path = "/x/v2/history/toview/add"
			}
			params = map[string]string{"aid": aid}
		case "favorite":
			aid, err := s.aid(ctx)
			if err != nil {
				return err
			}
			if folderIDs == nil && on {
				return bilibili.ErrInvalid
			}
			add, remove := []string{}, []string{}
			for _, id := range folderIDs {
				if !slices.ContainsFunc(before.Folders, func(f bilibili.ActionFolder) bool { return f.ID == id }) {
					return bilibili.ErrInvalid
				}
			}
			for _, f := range before.Folders {
				selected := slices.Contains(folderIDs, f.ID)
				if selected && !f.Selected {
					add = append(add, f.ID)
				}
				if !selected && f.Selected {
					remove = append(remove, f.ID)
				}
			}
			path, params = "/x/v3/fav/resource/deal", map[string]string{"rid": aid, "type": "2", "add_media_ids": strings.Join(add, ","), "del_media_ids": strings.Join(remove, ",")}
		}
	}
	_, err := s.bilibiliRequest(ctx, true, path, params)
	return err
}

func (s *actionSession) aid(ctx context.Context) (string, error) {
	// Resolve from view to bind the AV id to the requested BV id.
	body, err := s.bilibiliRequest(ctx, false, "/x/web-interface/view", map[string]string{"bvid": s.work.RemoteID})
	if err != nil {
		return "", err
	}
	return bilibili.ParseAID(body, s.work.RemoteID)
}

func (s *actionSession) bilibiliRequest(ctx context.Context, write bool, path string, params map[string]string) (body []byte, err error) {
	engine, ok := s.engine.(bilibiliActionEngine)
	if !ok {
		return nil, errors.New("UNSUPPORTED_SOURCE")
	}
	err = s.request(ctx, func() error {
		if write {
			body, err = engine.WriteBilibili(ctx, path, params)
		} else {
			body, err = engine.ReadBilibili(ctx, path, params)
		}
		return err
	})
	return
}

func (s *actionSession) douyinRequest(ctx context.Context, op douyin.Operation, params map[string]string) (reply douyin.PageReply, err error) {
	engine, ok := s.engine.(interface {
		Request(context.Context, douyin.Operation, map[string]string) (douyin.PageReply, error)
	})
	if !ok {
		return reply, errors.New("UNSUPPORTED_SOURCE")
	}
	err = s.request(ctx, func() error { reply, err = engine.Request(ctx, op, params); return err })
	return
}
