package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSchemaV5MigrationAndExtensions(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v4.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{schemaV1, schemaV2, schemaV3, schemaV4, `PRAGMA user_version = 4`} {
		if _, err := raw.ExecContext(ctx, script); err != nil {
			t.Fatalf("prepare v4 db: %v", err)
		}
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO accounts(id,source,remote_id,nickname,session_name,status,position,created_at,last_used_at) VALUES('acc-1','bilibili','u1','U1','s1','active',0,0,0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO works(id,source,remote_id,kind,title,published_at,updated_at) VALUES('bilibili:w1','bilibili','w1','video','W1',100,100)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO folders(account_id,remote_id,title,remote_count) VALUES('acc-1','f1','默认收藏夹',3)`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.ExecContext(ctx, `INSERT INTO work_account_state(account_id,work_id,liked,liked_at) VALUES('acc-1','bilibili:w1',1,100)`); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open and migrate v4->v5: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 5 {
		t.Fatalf("user_version=%d err=%v", version, err)
	}
	folders, err := s.Folders(ctx, "acc-1")
	if err != nil || len(folders) != 1 || folders[0].Kind != "favorite" || folders[0].Position != 0 {
		t.Fatalf("migrated folders=%+v err=%v", folders, err)
	}

	if err := s.SaveFolderMembership(ctx, "acc-1", Folder{
		ID: "f2", Title: "播单", Count: 1, Kind: "playlist", Position: 2, Extra: json.RawMessage(`{"public":true}`), UpdatedAt: 200,
	}, []FolderWork{{RemoteID: "w1", FavoritedAt: 200}}); err != nil {
		t.Fatalf("save extended folder: %v", err)
	}
	folders, err = s.Folders(ctx, "acc-1")
	if err != nil || len(folders) != 2 || folders[1].Kind != "playlist" || string(folders[1].Extra) != `{"public":true}` {
		t.Fatalf("extended folders=%+v err=%v", folders, err)
	}

	if err := s.SavePersonalState(ctx, "acc-1", "w1", PersonalState{Extra: json.RawMessage(`{"recommended":true}`)}); err != nil {
		t.Fatalf("save extra state: %v", err)
	}
	fav := true
	if err := s.SavePersonalState(ctx, "acc-1", "w1", PersonalState{Favorited: &fav}); err != nil {
		t.Fatalf("update favorited preserving extra: %v", err)
	}
	got, err := s.PersonalState(ctx, "acc-1", "w1")
	if err != nil || got.Liked == nil || !*got.Liked || got.Favorited == nil || !*got.Favorited || string(got.Extra) != `{"recommended":true}` {
		t.Fatalf("personal state=%+v err=%v", got, err)
	}
	if err := s.SavePersonalState(ctx, "acc-1", "w1", PersonalState{Extra: json.RawMessage(`{bad`)}); err == nil {
		t.Fatal("expected invalid JSON extra to fail")
	}
}
