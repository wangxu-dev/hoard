package bilibili

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/wangxu-dev/hoard/internal/store"
)

// Read fetches an API path inside the account's authenticated page.
// The caller owns session isolation and serial request scheduling.
type Read func(context.Context, string, map[string]string) ([]byte, error)

type Client struct{ Read Read }

func (c Client) read(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	call, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return c.Read(call, path, params)
}

// shortcut: indexing is additive; reconcile removals once complete snapshots are tracked.
// Index saves both lists before personal-state enrichment. It never infers
// removal from an incomplete traversal or from a failed state request.
func (c Client) Index(ctx context.Context, db *store.Store, accountID string) ([]Video, error) {
	account, err := db.Account(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("index account: %w", err)
	}
	if account.Source != "bilibili" || !numericID.MatchString(account.RemoteID) {
		return nil, fmt.Errorf("invalid Bilibili account")
	}
	body, err := c.read(ctx, "/x/v2/history/toview", nil)
	if err != nil {
		return nil, err
	}
	later, err := ParseWatchLater(body)
	if err != nil {
		return nil, err
	}
	if err = c.save(ctx, db, accountID, "watch_later", true, later); err != nil {
		return nil, err
	}
	body, err = c.read(ctx, "/x/v3/fav/folder/created/list-all", map[string]string{"up_mid": account.RemoteID})
	if err != nil {
		return nil, err
	}
	folders, err := ParseFolders(body)
	if err != nil {
		return nil, err
	}
	videos := append([]Video{}, later...)
	seen := map[string]bool{}
	for _, v := range later {
		seen[v.ID] = true
	}
	for _, f := range folders {
		folder := store.Folder{ID: f.ID, Title: f.Title, Count: f.Count}
		if err = db.SaveFolderMembership(ctx, accountID, folder, nil); err != nil {
			return nil, err
		}
		pagesSeen := map[[32]byte]bool{}
		for page := 1; ; page++ {
			if page > 10000 {
				return nil, fmt.Errorf("favorite pagination limit exceeded")
			}
			body, err = c.read(ctx, "/x/v3/fav/resource/list", map[string]string{"media_id": f.ID, "pn": strconv.Itoa(page), "ps": "20", "order": "mtime", "type": "0"})
			if err != nil {
				return nil, err
			}
			parsed, err := ParsePage(body)
			if err != nil {
				return nil, err
			}
			// A repeated response must not turn a broken pagination endpoint into a loop.
			key := sha256.Sum256(body)
			if pagesSeen[key] {
				return nil, fmt.Errorf("favorite pagination stalled")
			}
			pagesSeen[key] = true
			if err = c.save(ctx, db, accountID, "favorite", page == 1, parsed.Videos); err != nil {
				return nil, err
			}
			members := make([]store.FolderWork, 0, len(parsed.Videos))
			for _, v := range parsed.Videos {
				members = append(members, store.FolderWork{RemoteID: v.ID, FavoritedAt: v.FavoritedAt})
				if !seen[v.ID] {
					seen[v.ID] = true
					videos = append(videos, v)
				}
			}
			if err = db.SaveFolderMembership(ctx, accountID, folder, members); err != nil {
				return nil, err
			}
			if !parsed.HasMore {
				break
			}
		}
	}
	return videos, nil
}

func (c Client) save(ctx context.Context, db *store.Store, accountID, kind string, head bool, videos []Video) error {
	works := make([]store.Work, 0, len(videos))
	for _, v := range videos {
		extra, err := json.Marshal(struct {
			AID string `json:"aid"`
		}{v.AID})
		if err != nil {
			return err
		}
		w := store.Work{Extra: extra, RemoteID: v.ID, Kind: "video", Title: v.Title, CoverURL: v.Cover, PublishedAt: v.PublishedAt, DurationMS: v.DurationMS, Available: &v.Available}
		if v.AuthorID != "" {
			w.Author = &store.Author{RemoteID: v.AuthorID, Nickname: v.Author, AvatarURL: v.Avatar}
		}
		works = append(works, w)
	}
	_, err := db.SaveWorks(ctx, accountID, kind, head, works)
	return err
}

// Observe saves each confirmed field independently. Earlier observations survive
// a later endpoint failure, and callers can retry without changing membership.
func (c Client) Observe(ctx context.Context, db *store.Store, accountID string, video Video) error {
	account, err := db.Account(ctx, accountID)
	if err != nil {
		return fmt.Errorf("observe account: %w", err)
	}
	if account.Source != "bilibili" {
		return ErrInvalid
	}
	if !videoID.MatchString(video.ID) {
		return ErrInvalid
	}
	for _, kind := range []string{"like", "favorite", "coins"} {
		path, params := "/x/web-interface/archive/has/like", map[string]string{"bvid": video.ID}
		switch kind {
		case "favorite":
			if !numericID.MatchString(video.AID) || video.AID == "0" {
				return ErrInvalid
			}
			path, params = "/x/v2/fav/video/favoured", map[string]string{"aid": video.AID}
		case "coins":
			path = "/x/web-interface/archive/coins"
		}
		body, err := c.read(ctx, path, params)
		if err != nil {
			return err
		}
		var state store.PersonalState
		switch kind {
		case "like":
			state.Liked, err = ParseLiked(body)
		case "favorite":
			state.Favorited, err = ParseFavorited(body)
		case "coins":
			state.Coins, err = ParseCoins(body)
		}
		if err != nil {
			return err
		}
		if err = db.SavePersonalState(ctx, accountID, video.ID, state); err != nil {
			return err
		}
	}
	return nil
}
