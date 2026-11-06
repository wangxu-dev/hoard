package explore

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
)

// Diagnostics deliberately retain no paths, account/work IDs, URLs or arbitrary error text.
var diagnosticCodes = []string{"STORE_NOT_READY", "INVALID_REQUEST", "INVALID_SCOPE", "SCOPE_MISMATCH", "TASK_START_FAILED", "TASK_NOT_FOUND", "TASK_NOT_COMPLETE", "TASK_SUPERSEDED", "SERVICE_CLOSED", "SNAPSHOT_MISSING", "SNAPSHOT_INVALID", "SNAPSHOT_SCOPE_INVALID", "SNAPSHOT_TOO_LARGE", "SNAPSHOT_INCONSISTENT", "EXPLORATION_FAILED", "CANCELLED", "BRIDGE_OR_TRANSPORT", "UNKNOWN"}
var diagnosticPhrases = []string{"access is denied", "permission denied", "not a function", "unknown service", "operation not permitted", "拒绝访问", "系统找不到指定", "不支持", "未注册", "数据库已锁定", "no such file", "not found", "not registered", "not initialized", "not ready", "invalid handle", "incorrect function", "not supported", "read-only", "readonly", "locked", "busy", "timed out", "timeout", "cancelled", "canceled", "closed", "transport", "failed to fetch", "failed to execute", "method", "binding", "sqlite", "argument", "undefined", "null"}

func DiagnosticCode(message string) string {
	for _, code := range diagnosticCodes {
		if message == code {
			return code
		}
	}
	return "UNKNOWN"
}
func DiagnosticMessage(message string) string {
	message = strings.ToLower(message)
	var kept []string
	for _, phrase := range diagnosticPhrases {
		if strings.Contains(message, phrase) {
			kept = append(kept, phrase)
		}
	}
	if len(kept) == 0 {
		return "redacted"
	}
	return strings.Join(kept, "; ")
}
func LogDiagnostic(stage string, scope Scope, err error) {
	source := scope.Source
	if source != "douyin" && source != "bilibili" {
		if source == "" {
			source = "all"
		} else {
			source = "invalid"
		}
	}
	d := map[string]any{"stage": stage, "source": source, "accountScoped": scope.AccountID != ""}
	if err != nil {
		d["code"] = safeError(err)
		d["kind"] = fmt.Sprintf("%T", err)
		d["message"] = DiagnosticMessage(err.Error())
		var p *os.PathError
		if errors.As(err, &p) {
			switch p.Op {
			case "open", "sync", "write", "rename", "mkdir", "chmod", "close":
				d["operation"] = p.Op
			}
		}
		var errno syscall.Errno
		if errors.As(err, &errno) {
			d["errno"] = uint64(errno)
		}
	}
	b, _ := json.Marshal(d)
	log.Printf("[hoard:exploration] %s", b)
}
