package store

import (
	"context"
	"fmt"
)

// PlaybackAccounts returns every account that owns this work, across list kinds.
func (s *Store) PlaybackAccounts(ctx context.Context, source, remoteID string) ([]Account, error) {
	rows, err := s.read.QueryContext(ctx, `SELECT DISTINCT a.id,a.source,COALESCE(a.remote_id,''),a.nickname,COALESCE(a.avatar_url,''),a.session_name,a.status,a.position
 FROM accounts a JOIN relations r ON r.account_id=a.id JOIN works w ON w.id=r.work_id
 WHERE w.source=? AND w.remote_id=? ORDER BY a.position,a.id`, source, remoteID)
	if err != nil {
		return nil, fmt.Errorf("playback accounts: %w", err)
	}
	defer rows.Close()
	accounts := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Source, &a.RemoteID, &a.Nickname, &a.AvatarURL, &a.SessionName, &a.Status, &a.Position); err != nil {
			return nil, fmt.Errorf("read playback account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}
