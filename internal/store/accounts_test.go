package store

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestDatabaseAccountPrivacy(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	pending, err := s.CreatePendingAccount(ctx, "test-local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ConfirmAccount(ctx, pending.ID, "test-sec", "test-uid", "Test account", "https://example.test/avatar"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"accounts", "authors", "works", "relations", "musics", "music_relations", "sync_state", "blobs", "settings"} {
		t.Run(table, func(t *testing.T) {
			rows, err := s.db.Query(`SELECT * FROM ` + table)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			columns, err := rows.Columns()
			if err != nil {
				t.Fatal(err)
			}
			for _, column := range columns {
				if strings.Contains(strings.ToLower(column), "cookie") || strings.Contains(strings.ToLower(column), "token") {
					t.Fatal("database contains an authentication column")
				}
			}
			for rows.Next() {
				values := make([]any, len(columns))
				destinations := make([]any, len(columns))
				for i := range values {
					destinations[i] = &values[i]
				}
				if err := rows.Scan(destinations...); err != nil {
					t.Fatal(err)
				}
				for _, value := range values {
					text := strings.ToLower(fmt.Sprint(value))
					for _, forbidden := range []string{"cookie", "token", "session-only-marker"} {
						if strings.Contains(text, forbidden) {
							t.Fatal("database contains browser session data")
						}
					}
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConfirmAccounts(t *testing.T) {
	for _, tt := range []struct {
		name   string
		legacy bool
	}{
		{"same sec identity", false}, {"previous UID identity", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "library.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			first, err := s.CreatePendingAccount(ctx, "test-first")
			if err != nil {
				t.Fatal(err)
			}
			remote := "test-sec"
			if tt.legacy {
				remote = "test-uid"
			}
			first, err = s.ConfirmAccount(ctx, first.ID, remote, "", "First", "https://example.test/avatar")
			if err != nil {
				t.Fatal(err)
			}
			if err := s.WriteWorkPage(ctx, WorkPage{AccountID: first.ID, Kind: "like", Phase: "head", Cursor: "1", Works: []Work{{RemoteID: "test-work", Kind: "video"}}}); err != nil {
				t.Fatal(err)
			}
			pending, err := s.CreatePendingAccount(ctx, "test-second")
			if err != nil {
				t.Fatal(err)
			}
			if first.SessionName == pending.SessionName {
				t.Fatal("sessions share a name")
			}
			if err := s.ActivateAccount(ctx, pending.ID); err != nil {
				t.Fatal(err)
			}
			confirmed, err := s.ConfirmAccount(ctx, pending.ID, "test-sec", "test-uid", "Updated", "https://example.test/updated")
			if err != nil {
				t.Fatal(err)
			}
			if confirmed.ID != first.ID || confirmed.RemoteID != "test-sec" || confirmed.AvatarURL != "https://example.test/updated" || confirmed.SessionName != pending.SessionName {
				t.Fatal("identity or session was not reused")
			}
			accounts, err := s.Accounts(ctx)
			if err != nil || len(accounts) != 1 {
				t.Fatal("duplicate account remains")
			}
			if n, err := s.CountWorks(ctx, first.ID, "like"); err != nil || n != 1 {
				t.Fatal("local relations lost")
			}
			if id, err := s.Setting(ctx, "active_account"); err != nil || id != first.ID {
				t.Fatal("active selection was not reused")
			}
			cleanup, err := s.SessionCleanups(ctx)
			if err != nil || cleanup[first.ID] != first.SessionName {
				t.Fatal("temporary session cleanup was not queued")
			}
		})
	}
}
