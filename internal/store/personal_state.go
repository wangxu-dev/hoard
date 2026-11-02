package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PersonalState observations are independent of list membership and public counters.
type PersonalState struct {
	Liked       *bool
	Favorited   *bool
	Coins       *int
	LikedAt     *int64
	FavoritedAt *int64
	CoinsAt     *int64
}

func (s *Store) SavePersonalState(ctx context.Context, accountID, remoteID string, state PersonalState) error {
	if state.Coins != nil && (*state.Coins < 0 || *state.Coins > 2) {
		return fmt.Errorf("invalid coin amount")
	}
	if state.Liked == nil && state.Favorited == nil && state.Coins == nil {
		return nil
	}
	now := time.Now().Unix()
	result, err := s.db.ExecContext(ctx, `INSERT INTO work_account_state(account_id,work_id,liked,liked_at,favorited,favorited_at,coins,coins_at)
 SELECT a.id,w.id,?,CASE WHEN ? IS NOT NULL THEN ? END,?,CASE WHEN ? IS NOT NULL THEN ? END,?,CASE WHEN ? IS NOT NULL THEN ? END
 FROM accounts a JOIN works w ON w.source=a.source WHERE a.id=? AND w.remote_id=?
 ON CONFLICT(account_id,work_id) DO UPDATE SET
 liked=COALESCE(excluded.liked,liked),liked_at=COALESCE(excluded.liked_at,liked_at),
 favorited=COALESCE(excluded.favorited,favorited),favorited_at=COALESCE(excluded.favorited_at,favorited_at),
 coins=COALESCE(excluded.coins,coins),coins_at=COALESCE(excluded.coins_at,coins_at)`, state.Liked, state.Liked, now, state.Favorited, state.Favorited, now, state.Coins, state.Coins, now, accountID, remoteID)
	if err != nil {
		return fmt.Errorf("save personal state: %w", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		return fmt.Errorf("personal state target missing")
	}
	return nil
}
func (s *Store) PersonalState(ctx context.Context, accountID, remoteID string) (PersonalState, error) {
	var state PersonalState
	err := s.read.QueryRowContext(ctx, `SELECT s.liked,s.favorited,s.coins,s.liked_at,s.favorited_at,s.coins_at FROM work_account_state s JOIN works w ON w.id=s.work_id WHERE s.account_id=? AND w.remote_id=?`, accountID, remoteID).Scan(&state.Liked, &state.Favorited, &state.Coins, &state.LikedAt, &state.FavoritedAt, &state.CoinsAt)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	return state, err
}
