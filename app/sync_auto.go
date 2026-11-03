package app

import "github.com/wangxu-dev/hoard/internal/store"

func automaticSyncDue(source string, records []store.SyncRecord, now int64) bool {
	kinds := syncKinds
	if source == "bilibili" {
		kinds = []string{"favorite", "watch_later"}
	}
	for _, kind := range kinds {
		var record *store.SyncRecord
		for i := range records {
			if records[i].Kind == kind {
				record = &records[i]
				break
			}
		}
		if record == nil {
			return true
		}
		if record.Status == "unsupported" {
			continue
		}
		if record.Phase != "done" || record.LastFullAt == nil || now-*record.LastFullAt > 180 {
			return true
		}
	}
	return false
}
