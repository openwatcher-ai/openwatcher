package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveWritesPrivateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Config{
		TokenHash:     "hash",
		PublicBaseURL: "https://example.test/",
	}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 0600", got)
	}

	loaded, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Listen != DefaultListen {
		t.Fatalf("Listen = %q, want default %q", loaded.Listen, DefaultListen)
	}
	if loaded.PublicBaseURL != "https://example.test" {
		t.Fatalf("PublicBaseURL = %q, want trimmed value", loaded.PublicBaseURL)
	}
	if loaded.ApkDistDir != DefaultApkDistDir {
		t.Fatalf("ApkDistDir = %q, want default %q", loaded.ApkDistDir, DefaultApkDistDir)
	}
	if loaded.DevUpdateAllowlist != DefaultDevUpdateAllowlist {
		t.Fatalf("DevUpdateAllowlist = %q, want default %q", loaded.DevUpdateAllowlist, DefaultDevUpdateAllowlist)
	}
	if loaded.ScreenshotUploadDir != DefaultScreenshotUploadDir {
		t.Fatalf("ScreenshotUploadDir = %q, want default %q", loaded.ScreenshotUploadDir, DefaultScreenshotUploadDir)
	}
	if loaded.DiagnosticUploadDir != DefaultDiagnosticUploadDir {
		t.Fatalf("DiagnosticUploadDir = %q, want default %q", loaded.DiagnosticUploadDir, DefaultDiagnosticUploadDir)
	}
}

func TestEffectivePublicBaseURLUsesListenWhenUnset(t *testing.T) {
	cfg := Config{Listen: "0.0.0.0:18787"}
	cfg.ApplyDefaults()
	if got := cfg.EffectivePublicBaseURL(); got != "http://127.0.0.1:18787" {
		t.Fatalf("EffectivePublicBaseURL() = %q", got)
	}
}

func TestApplyDefaultsMigratesLegacyPairingIntoBetaSlot(t *testing.T) {
	cfg := Config{
		TokenHash:  "beta-hash",
		DeviceName: "watch",
		PairedAt:   "2026-06-09T12:00:00Z",
	}

	cfg.ApplyDefaults()

	if got := cfg.PairingForSlot(PairingSlotBeta).TokenHash; got != "beta-hash" {
		t.Fatalf("beta pairing token hash = %q", got)
	}
	if cfg.BetaPairing == nil {
		t.Fatalf("expected beta pairing to be materialized")
	}
}

func TestSetPairingForSlotKeepsLegacyBetaFields(t *testing.T) {
	cfg := Config{}

	cfg.SetPairingForSlot(PairingSlotBeta, "beta-hash", "beta-watch", "2026-06-09T12:00:00Z")
	cfg.SetPairingForSlot(PairingSlotDev, "dev-hash", "dev-watch", "2026-06-09T13:00:00Z")

	if cfg.TokenHash != "beta-hash" || cfg.DeviceName != "beta-watch" || cfg.PairedAt != "2026-06-09T12:00:00Z" {
		t.Fatalf("legacy beta fields not mirrored: %#v", cfg)
	}
	if got := cfg.TokenHashForSlot(PairingSlotDev); got != "dev-hash" {
		t.Fatalf("dev token hash = %q", got)
	}
}
