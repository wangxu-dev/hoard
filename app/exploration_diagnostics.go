package app

import (
	"encoding/json"
	"github.com/wangxu-dev/hoard/internal/explore"
	"log"
)

// ExplorationDiagnostic accepts only bounded, redacted UI-stage information.
type ExplorationDiagnostic struct {
	Stage         string `json:"stage"`
	Source        string `json:"source"`
	AccountScoped bool   `json:"accountScoped"`
	Sequence      int    `json:"sequence"`
	Code          string `json:"code"`
	Kind          string `json:"kind"`
	Message       string `json:"message"`
	State         string `json:"state"`
}

// ReportExplorationDiagnostic uses the existing App bridge and native log even if Exploration.Start rejects.
func (a *App) ReportExplorationDiagnostic(d ExplorationDiagnostic) {
	stages := map[string]bool{"request": true, "start-call": true, "start-ok": true, "start-reject": true, "status-call": true, "status-state": true, "status-reject": true, "result-call": true, "result-ok": true, "result-reject": true, "scope-reject": true, "abort": true, "late-drop": true, "failed": true}
	if !stages[d.Stage] {
		d.Stage = "unknown"
	}
	if d.Source != "douyin" && d.Source != "bilibili" {
		d.Source = "all"
	}
	if d.Sequence < 0 || d.Sequence > 1000000 {
		d.Sequence = 0
	}
	d.Code = explore.DiagnosticCode(d.Code)
	d.Message = explore.DiagnosticMessage(d.Message)
	switch d.Kind {
	case "Error", "TypeError", "AbortError", "string", "object", "none":
	default:
		d.Kind = "unknown"
	}
	switch d.State {
	case "reading", "analyzing", "complete", "failed", "cancelled", "superseded":
	default:
		d.State = ""
	}
	a.mu.Lock()
	storeReady := a.Store != nil
	engineReady := a.Engine != nil && a.Engine.Status().Ready
	a.mu.Unlock()
	b, _ := json.Marshal(struct {
		ExplorationDiagnostic
		StoreReady  bool `json:"storeReady"`
		EngineReady bool `json:"engineReady"`
	}{d, storeReady, engineReady})
	log.Printf("[hoard:exploration-ui] %s", b)
}
