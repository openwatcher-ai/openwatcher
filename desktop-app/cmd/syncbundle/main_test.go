package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWidgetTargetsMatchRuntimeLocatorLayout(t *testing.T) {
	appPath := filepath.Join(t.TempDir(), "OpenWatcher.app")
	mac := widgetTargets(filepath.Join(appPath, "Contents", "Resources", "bundled"), appPath, "darwin", "arm64")
	wantApp := filepath.Join(appPath, "Contents", "Library", "Helpers", "OpenWatcher Widget.app")
	if mac.AppBundle != wantApp || mac.Executable != filepath.Join(wantApp, "Contents", "MacOS", "openwatcher-widget") {
		t.Fatalf("mac targets = %+v", mac)
	}
	bundled := filepath.Join(t.TempDir(), "bundled")
	windows := widgetTargets(bundled, "", "windows", "amd64")
	wantWindows := filepath.Join(bundled, "widget", "windows-amd64", "openwatcher-widget.exe")
	if windows.AppBundle != "" || windows.Executable != wantWindows {
		t.Fatalf("Windows targets = %+v", windows)
	}
}

func TestWriteWidgetInfoPlistMarksAccessoryApplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "OpenWatcher Widget.app", "Contents", "Info.plist")
	if err := writeWidgetInfoPlist(path, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		"<string>ai.openwatcher.widget</string>",
		"<key>LSUIElement</key><true/>",
		"<key>CFBundleVersion</key><string>1.2.3</string>",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("Info.plist missing %q: %s", required, text)
		}
	}
}

func TestWidgetBundleVersionRejectsNonReleaseValues(t *testing.T) {
	for raw, want := range map[string]string{"": "0.0.0", "v1.2.3": "1.2.3", "2.0": "0.0.0", "dev-local": "0.0.0", "1.2.3-beta.2": "1.2.3"} {
		t.Setenv("OPENWATCHER_DESKTOP_VERSION", raw)
		if got := widgetBundleVersion(); got != want {
			t.Fatalf("widgetBundleVersion(%q) = %q, want %q", raw, got, want)
		}
	}
}
