package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wangxu-dev/hoard/internal/douyin"
	"github.com/wangxu-dev/hoard/internal/store"
)

func TestSyncFoldersReplacesAndPrunes(t *testing.T) {
	a, _, eng := newAccountTestApp(t)
	ctx := context.Background()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveWorks(ctx, "test", "favorite", false, []store.Work{
		{RemoteID: "101", Kind: "video", Title: "One"},
		{RemoteID: "102", Kind: "video", Title: "Two"},
	}); err != nil {
		t.Fatal(err)
	}
	// Pre-existing folder state: f1 has stale 101, f-stale should be pruned.
	if err := a.Store.ReplaceFolderMembership(ctx, "test", store.Folder{ID: "f1", Title: "Old One", Count: 1, Position: 0}, []store.FolderWork{{RemoteID: "101"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.ReplaceFolderMembership(ctx, "test", store.Folder{ID: "f-stale", Title: "Stale", Count: 1, Position: 1}, []store.FolderWork{{RemoteID: "102"}}); err != nil {
		t.Fatal(err)
	}

	a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(op douyin.Operation, p map[string]string) (douyin.PageReply, error) {
		a.lastRequest = time.Time{}
		switch op {
		case douyin.OpFolders:
			if p["cursor"] == "0" {
				return douyin.PageReply{Status: 200, Body: `{"status_code":0,"has_more":1,"cursor":"1","collects_list":[{"collects_id_str":"f1","collects_name":"Folder One","total_number":1}]}`}, nil
			}
			return douyin.PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"cursor":"2","collects_list":[{"collects_id_str":"f2","collects_name":"Folder Two","total_number":1}]}`}, nil
		case douyin.OpFolderItems:
			switch p["collects_id"] {
			case "f1":
				return douyin.PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"cursor":"0","aweme_list":[{"aweme_id":"102","desc":"Two"}]}`}, nil
			case "f2":
				return douyin.PageReply{Status: 200, Body: `{"status_code":0,"has_more":0,"cursor":"0","aweme_list":[{"aweme_id":"103","desc":"Three"}]}`}, nil
			}
		}
		return douyin.PageReply{}, fmt.Errorf("unexpected op %s", op)
	}}

	s := NewSync(a)
	s.syncFolders(ctx, "test")

	folders, err := a.Store.Folders(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 2 || folders[0].ID != "f1" || folders[0].Title != "Folder One" || folders[1].ID != "f2" || folders[1].Position != 1 {
		t.Fatalf("folders = %+v", folders)
	}
	m1, err := a.Store.FolderMembers(ctx, "test", "f1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m1["102"]; !ok || len(m1) != 1 {
		t.Fatalf("f1 members = %v", m1)
	}
	m2, err := a.Store.FolderMembers(ctx, "test", "f2")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m2["103"]; !ok || len(m2) != 1 {
		t.Fatalf("f2 members = %v", m2)
	}
	stale, err := a.Store.FolderMembers(ctx, "test", "f-stale")
	if err != nil || len(stale) != 0 {
		t.Fatalf("stale folder members still present: %v, err=%v", stale, err)
	}
}

func TestSyncFoldersSkipsPruneWhenFolderListFails(t *testing.T) {
	a, _, eng := newAccountTestApp(t)
	ctx := context.Background()
	if _, err := a.Store.UpsertAccount(ctx, store.Account{ID: "test", Source: "douyin", RemoteID: "42", SessionName: "test", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.SaveWorks(ctx, "test", "favorite", false, []store.Work{{RemoteID: "101", Kind: "video", Title: "One"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.ReplaceFolderMembership(ctx, "test", store.Folder{ID: "f1", Title: "Keep", Count: 1, Position: 0}, []store.FolderWork{{RemoteID: "101"}}); err != nil {
		t.Fatal(err)
	}

	a.Engine = &syncTestEngine{fakeAccountEngine: eng, request: func(op douyin.Operation, p map[string]string) (douyin.PageReply, error) {
		a.lastRequest = time.Time{}
		if op == douyin.OpFolders && p["cursor"] == "0" {
			return douyin.PageReply{Status: 200, Body: `{"status_code":0,"has_more":1,"cursor":"1","collects_list":[{"collects_id_str":"f2","collects_name":"Two","total_number":0}]}`}, nil
		}
		return douyin.PageReply{Status: 200, Body: `{"status_code":2154}`}, nil
	}}

	s := NewSync(a)
	s.syncFolders(ctx, "test")

	folders, err := a.Store.Folders(ctx, "test")
	if err != nil || len(folders) != 1 || folders[0].ID != "f1" {
		t.Fatalf("expected f1 preserved on folder list error, got %+v, err=%v", folders, err)
	}
}
