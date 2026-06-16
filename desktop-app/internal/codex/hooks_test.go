package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstallOpenWatcherHooksCreatesUserHooksFile(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	binaryPath := filepath.Join(t.TempDir(), "openwatcher")
	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)

	status, err := installOpenWatcherHooks(binaryPath, now)
	if err != nil {
		t.Fatalf("installOpenWatcherHooks err = %v", err)
	}
	if !status.Installed || !status.Changed {
		t.Fatalf("status = %+v", status)
	}
	if status.BackupPath != "" {
		t.Fatalf("BackupPath = %q, want empty for new file", status.BackupPath)
	}

	root := readTestHooks(t, filepath.Join(codexHome, "hooks.json"))
	if !hasOpenWatcherHook(root, "PreCompact") || !hasOpenWatcherHook(root, "PostCompact") {
		t.Fatalf("installed hooks missing: %#v", root)
	}
}

func TestInstallOpenWatcherHooksPreservesExistingHooksAndCreatesBackup(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	hooksPath := filepath.Join(codexHome, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{
  "hooks": {
    "PreCompact": [
      {
        "matcher": "manual",
        "hooks": [
          {
            "type": "command",
            "command": "echo user",
            "timeout": 2
          }
        ]
      }
    ]
  }
}
`), 0o600); err != nil {
		t.Fatalf("write hooks: %v", err)
	}

	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)
	status, err := installOpenWatcherHooks(filepath.Join(t.TempDir(), "openwatcher"), now)
	if err != nil {
		t.Fatalf("installOpenWatcherHooks err = %v", err)
	}
	if status.BackupPath == "" {
		t.Fatalf("BackupPath empty")
	}
	if _, err := os.Stat(status.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}

	root := readTestHooks(t, hooksPath)
	preGroups := root["hooks"].(map[string]any)["PreCompact"].([]any)
	if len(preGroups) != 2 {
		t.Fatalf("PreCompact groups = %d, want 2", len(preGroups))
	}
	if !containsCommand(preGroups, "echo user") {
		t.Fatalf("user hook was not preserved: %#v", preGroups)
	}
	if !hasOpenWatcherHook(root, "PreCompact") || !hasOpenWatcherHook(root, "PostCompact") {
		t.Fatalf("OpenWatcher hook missing: %#v", root)
	}
}

func TestInstallOpenWatcherHooksRejectsInvalidJSON(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	hooksPath := filepath.Join(codexHome, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(`{bad`), 0o600); err != nil {
		t.Fatalf("write hooks: %v", err)
	}

	if _, err := installOpenWatcherHooks(filepath.Join(t.TempDir(), "openwatcher"), time.Now()); err == nil {
		t.Fatalf("installOpenWatcherHooks err = nil, want error")
	}
	got, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks: %v", err)
	}
	if string(got) != "{bad" {
		t.Fatalf("hooks changed after invalid JSON: %s", got)
	}
}

func readTestHooks(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hooks: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("decode hooks: %v", err)
	}
	return root
}

func containsCommand(groups []any, command string) bool {
	for _, rawGroup := range groups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		for _, rawHook := range asSlice(group["hooks"]) {
			hook, ok := rawHook.(map[string]any)
			if ok && hook["command"] == command {
				return true
			}
		}
	}
	return false
}
