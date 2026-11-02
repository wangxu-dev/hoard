package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	for _, tt := range []struct {
		name    string
		setup   string
		wantErr bool
	}{
		{name: "empty"},
		{name: "prototype", setup: `
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
INSERT INTO settings VALUES ('uid', 'test-account');
CREATE TABLE accounts (uid TEXT PRIMARY KEY, nickname TEXT NOT NULL, cursor TEXT NOT NULL, has_more INTEGER NOT NULL);
INSERT INTO accounts VALUES ('test-account', 'Test account', '99', 1);
CREATE TABLE works (uid TEXT NOT NULL, id TEXT NOT NULL, desc TEXT NOT NULL, author TEXT NOT NULL, cover TEXT NOT NULL, duration INTEGER NOT NULL, created_at INTEGER NOT NULL, aweme_type INTEGER NOT NULL, play_url TEXT NOT NULL, images TEXT NOT NULL, PRIMARY KEY(uid,id));
CREATE INDEX works_created ON works(uid,created_at DESC);
INSERT INTO works VALUES ('test-account','test-work','Test work','Test author','',0,1,0,'','[]');`},
		{name: "future", setup: `PRAGMA user_version=2`, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "library.db")+"?_pragma=foreign_keys(ON)")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if tt.setup != "" {
				if _, err := db.Exec(tt.setup); err != nil {
					t.Fatal(err)
				}
			}
			err = migrate(context.Background(), db)
			if (err != nil) != tt.wantErr {
				t.Fatalf("migration error: %v", err)
			}
			if tt.wantErr {
				return
			}
			for _, table := range []string{"accounts", "authors", "works", "musics", "relations", "music_relations", "sync_state", "blobs", "settings"} {
				var count int
				if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil || count != 0 {
					t.Fatalf("%s: count=%d err=%v", table, count, err)
				}
			}
			var version int
			if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 1 {
				t.Fatalf("version=%d err=%v", version, err)
			}
			if _, err := db.Exec(`INSERT INTO settings VALUES ('test', 'kept')`); err != nil {
				t.Fatal(err)
			}
			if err := migrate(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			var value string
			if err := db.QueryRow(`SELECT value FROM settings WHERE key='test'`).Scan(&value); err != nil || value != "kept" {
				t.Fatalf("repeat migration: %q %v", value, err)
			}
		})
	}
}
