package explore

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestExploreDiagnosticRedactsPrivateValues(t *testing.T) {
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(old)
	LogDiagnostic("cache-save-failed", Scope{AccountID: "private-account-id", Source: "douyin"}, &os.PathError{Op: "sync", Path: "C:/Users/private-name/private-title", Err: syscall.EACCES})
	text := output.String()
	for _, v := range []string{"private-account-id", "private-name", "private-title", "C:/Users"} {
		if strings.Contains(text, v) {
			t.Fatal("private data leaked")
		}
	}
	for _, v := range []string{"cache-save-failed", "operation", "sync", "errno", "accountScoped"} {
		if !strings.Contains(text, v) {
			t.Fatalf("missing safe diagnostic %s", v)
		}
	}
	output.Reset()
	LogDiagnostic("start", Scope{AccountID: "private-account-id", Source: "private-source"}, errors.New("secret-token user-name permission denied"))
	text = output.String()
	if strings.Contains(text, "private-source") || strings.Contains(text, "secret-token") || strings.Contains(text, "user-name") {
		t.Fatal("unsafe error logged")
	}
	if !strings.Contains(text, "permission denied") {
		t.Fatal("safe message lost")
	}
	if DiagnosticMessage("姓名 title secret-token https://private.invalid") != "redacted" {
		t.Fatal("unknown text retained")
	}
}
