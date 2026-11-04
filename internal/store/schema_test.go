package store

import (
	"context"
	"testing"
)

func TestSchemaConstraints(t *testing.T) {
	for _, tt := range []struct {
		name      string
		statement string
	}{
		{"unique remote identity", `INSERT INTO accounts(id,source,remote_id,session_name,status,position,created_at,last_used_at) VALUES('duplicate','alpha','remote-account-a','duplicate-session','active',0,0,0)`},
		{"unique session", `INSERT INTO accounts(id,source,remote_id,session_name,status,position,created_at,last_used_at) VALUES('duplicate','beta','other','session-account-a','active',0,0,0)`},
		{"author avatar foreign key", `INSERT INTO authors(id,source,remote_id,nickname,avatar_hash,updated_at) VALUES('alpha:author','alpha','author','Author','missing',0)`},
		{"work author foreign key", `INSERT INTO works(id,source,remote_id,kind,title,author_id,published_at,updated_at) VALUES('alpha:work','alpha','work','video','Work','missing',0,0)`},
		{"relation foreign key", `INSERT INTO relations(account_id,work_id,kind,seq,published_at,first_seen) VALUES('account-a','missing','like',1,0,0)`},
		{"music cover foreign key", `INSERT INTO musics(id,source,remote_id,title,author,cover_hash,updated_at) VALUES('alpha:music','alpha','music','Music','Author','missing',0)`},
		{"music relation foreign key", `INSERT INTO music_relations(account_id,music_id,seq,first_seen) VALUES('account-a','missing',1,0)`},
		{"sync account foreign key", `INSERT INTO sync_state(account_id,kind,phase,status,updated_at) VALUES('missing','like','head','running',0)`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			addTestAccount(t, s, "account-a", "alpha")
			if _, err := s.db.Exec(tt.statement); err == nil {
				t.Fatal("expected constraint failure")
			}
		})
	}
}

func TestDatabaseConfiguration(t *testing.T) {
	s := newTestStore(t)
	for _, tt := range []struct{ pragma, want string }{
		{"journal_mode", "wal"}, {"foreign_keys", "1"}, {"busy_timeout", "5000"}, {"user_version", "4"}, {"synchronous", "2"},
	} {
		t.Run(tt.pragma, func(t *testing.T) {
			var value string
			if err := s.db.QueryRow(`PRAGMA ` + tt.pragma).Scan(&value); err != nil || value != tt.want {
				t.Fatalf("%s=%q err=%v", tt.pragma, value, err)
			}
		})
	}
	for _, index := range []string{"works_published", "works_author", "relations_list", "relations_time", "relations_work"} {
		t.Run(index, func(t *testing.T) {
			var name string
			if err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name=?`, index).Scan(&name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInvalidPage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*WorkPage)
	}{
		{"kind", func(p *WorkPage) { p.Kind = "music" }},
		{"phase", func(p *WorkPage) { p.Phase = "idle" }},
		{"account", func(p *WorkPage) { p.AccountID = "missing" }},
		{"remote id", func(p *WorkPage) { p.Works[0].RemoteID = "" }},
		{"work kind", func(p *WorkPage) { p.Works[0].Kind = "audio" }},
		{"author id", func(p *WorkPage) { p.Works[0].Author.RemoteID = "" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			id := addTestAccount(t, s, "account-a", "alpha")
			p := WorkPage{AccountID: id, Kind: "like", Phase: "head", Cursor: "next", Works: []Work{testWork("1")}}
			tt.change(&p)
			if err := s.WriteWorkPage(context.Background(), p); err == nil {
				t.Fatal("expected invalid page")
			}
			assertRows(t, s, "works", 0)
			assertRows(t, s, "authors", 0)
			assertRows(t, s, "relations", 0)
			assertRows(t, s, "sync_state", 0)
		})
	}
}

func TestInvalidPageLimit(t *testing.T) {
	for _, limit := range []int{-1, 0, 1001} {
		s := newTestStore(t)
		if _, _, err := s.ListWorks(context.Background(), "test", "like", nil, limit); err == nil {
			t.Fatalf("accepted limit %d", limit)
		}
	}
}
