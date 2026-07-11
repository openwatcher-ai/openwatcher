package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFloatingWidgetMigrationDistinguishesMissingFileAndMissingField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	got, err := LoadDesktopSettings()
	if err != nil || !got.FloatingWidgetEnabled {
		t.Fatalf("new defaults=%+v err=%v", got, err)
	}
	p, err := desktopSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"autoStartBackend":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = LoadDesktopSettings()
	if err != nil || got.FloatingWidgetEnabled {
		t.Fatalf("upgrade=%+v err=%v", got, err)
	}
	if err := os.WriteFile(p, []byte(`{"floatingWidgetEnabled":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ = LoadDesktopSettings()
	if !got.FloatingWidgetEnabled {
		t.Fatal("explicit true lost")
	}
	if err := os.WriteFile(p, []byte(`{"floatingWidgetEnabled":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ = LoadDesktopSettings()
	if got.FloatingWidgetEnabled {
		t.Fatal("explicit false lost")
	}
}
