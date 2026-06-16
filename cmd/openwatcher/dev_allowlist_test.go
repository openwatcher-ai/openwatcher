package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

func TestDevAllowlistListShowsCurrentAndHistory(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{
		TokenHash:  pairing.HashToken(token),
		DeviceName: "Xiaomi Watch 5",
		PairedAt:   "2026-06-09T12:00:00Z",
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	if err := pairing.RecordBinding(pairing.HistoryPath(configPath), pairing.BindingRecord{
		TokenHash:  pairing.HashToken("1123456789abcdef0123456789abcdef"),
		DeviceName: "Galaxy Watch",
		PairedAt:   "2026-06-08T12:00:00Z",
		Source:     "pair-page",
	}); err != nil {
		t.Fatalf("RecordBinding() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := devAllowlistCommand{stdout: &stdout, stderr: &stderr}
	handled, exitCode := command.maybeRun([]string{"openwatcher", "dev-allowlist", "list", "--config", configPath})
	if !handled || exitCode != 0 {
		t.Fatalf("handled=%v exitCode=%d stderr=%s", handled, exitCode, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Xiaomi Watch 5") || !strings.Contains(output, "Galaxy Watch") {
		t.Fatalf("list output = %s", output)
	}
}

func TestDevAllowlistAddByIndexWritesAllowlist(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{
		TokenHash:          pairing.HashToken(token),
		DeviceName:         "Xiaomi Watch 5",
		PairedAt:           "2026-06-09T12:00:00Z",
		DevUpdateAllowlist: ".tmp/openwatcher-dev-update-allowlist.txt",
	}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := devAllowlistCommand{stdout: &stdout, stderr: &stderr}
	handled, exitCode := command.maybeRun([]string{"openwatcher", "dev-allowlist", "add", "--config", configPath, "--index", "1"})
	if !handled || exitCode != 0 {
		t.Fatalf("handled=%v exitCode=%d stderr=%s", handled, exitCode, stderr.String())
	}
	allowlistPath := pairing.ResolveRelativeToConfig(configPath, cfg.DevUpdateAllowlist)
	entries, err := pairing.LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist() error = %v", err)
	}
	if len(entries) != 1 || entries[0] != pairing.HashToken(token) {
		t.Fatalf("allowlist entries = %#v", entries)
	}
}
