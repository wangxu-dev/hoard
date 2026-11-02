package douyin

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
)

var workIDPattern = regexp.MustCompile(`^\d{1,30}$`)

func ValidWorkID(id string) bool { return workIDPattern.MatchString(id) }

// ActionState only accepts explicit per-user state, never aggregate counts.
type ActionState struct {
	Liked     bool   `json:"liked"`
	Favorited bool   `json:"favorited"`
	AwemeType string `json:"-"`
}

func ParseActionState(reply PageReply, id string) (ActionState, error) {
	obj, class, _ := classify(OpDetail, reply)
	if class != ResponseOK {
		return ActionState{}, errors.New(string(class))
	}
	var detail struct {
		ID       json.Number     `json:"aweme_id"`
		Type     json.Number     `json:"aweme_type"`
		Like     json.RawMessage `json:"user_digged"`
		Favorite json.RawMessage `json:"collect_stat"`
	}
	if json.Unmarshal(obj["aweme_detail"], &detail) != nil || string(detail.ID) != id {
		return ActionState{}, errors.New("STATE_UNKNOWN")
	}
	state := ActionState{AwemeType: string(detail.Type)}
	read := func(raw json.RawMessage) (bool, bool) {
		switch string(raw) {
		case "0", `"0"`:
			return false, true
		case "1", `"1"`:
			return true, true
		}
		return false, false
	}
	var ok bool
	state.Liked, ok = read(detail.Like)
	if !ok {
		return ActionState{}, errors.New("STATE_UNKNOWN")
	}
	state.Favorited, ok = read(detail.Favorite)
	if !ok {
		return ActionState{}, errors.New("STATE_UNKNOWN")
	}
	if _, err := strconv.ParseUint(state.AwemeType, 10, 32); err != nil {
		return ActionState{}, errors.New("STATE_UNKNOWN")
	}
	return state, nil
}
