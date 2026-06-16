package codex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectReportsDetectedFiles(t *testing.T) {
	temp := t.TempDir()
	if err := os.WriteFile(filepath.Join(temp, "auth.json"), []byte(`{"tokens":{"access_token":"secret"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	if err := os.Mkdir(filepath.Join(temp, "sessions"), 0o700); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}

	old := os.Getenv("CODEX_HOME")
	if err := os.Setenv("CODEX_HOME", temp); err != nil {
		t.Fatalf("set env: %v", err)
	}
	defer func() {
		_ = os.Setenv("CODEX_HOME", old)
	}()

	status := NewDetector().Inspect()
	if !status.AuthDetected || !status.SessionsDetected || !status.Readable {
		t.Fatalf("unexpected codex status: %#v", status)
	}
	if status.HomeLabel != "CODEX_HOME" {
		t.Fatalf("home label = %q, want CODEX_HOME", status.HomeLabel)
	}
}
