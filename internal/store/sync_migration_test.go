package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateV3PreservesSyncState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, schema := range []string{schemaV1, schemaV2, schemaV3} {
		if _, err = db.Exec(schema); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO accounts(id,source,remote_id,session_name,status,position,created_at,last_used_at) VALUES('test','douyin','42','test','active',0,0,0); INSERT INTO sync_state(account_id,kind,phase,status,backfill_cursor,retry_at,failures,updated_at) VALUES('test','like','backfill','cooling','next',9999999999,2,0); PRAGMA user_version=3`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	records, err := s.SyncRecords(context.Background(), "test")
	if err != nil || len(records) != 1 {
		t.Fatalf("records %+v %v", records, err)
	}
	r := records[0]
	if r.BackfillCursor != "next" || r.RetryAt == nil || *r.RetryAt != 9999999999 || r.Failures != 2 || r.DelayMS != 0 {
		t.Fatal(r)
	}
}
