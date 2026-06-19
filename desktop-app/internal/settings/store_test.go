package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	tempDir := t.TempDir()
	appRoot := filepath.Join(tempDir, "repo", "desktop-app")
	nestedDir := filepath.Join(appRoot, "build", "bin", "OpenWatcher.app", "Contents", "MacOS")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "wails.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write wails.json: %v", err)
	}

	if got := findProjectRoot(nestedDir); got != appRoot {
		t.Fatalf("findProjectRoot() = %q, want %q", got, appRoot)
	}
}

func TestRuntimeResourceRootsPrefersRuntimeDir(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	roots := RuntimeResourceRoots(appRoot)
	if len(roots) < 2 {
		t.Fatalf("RuntimeResourceRoots len = %d", len(roots))
	}
	if !strings.HasSuffix(roots[0], filepath.Join("OpenWatcher", "runtime")) && !strings.HasSuffix(roots[0], filepath.Join("openwatcher", "runtime")) {
		t.Fatalf("first runtime root = %q", roots[0])
	}
	wantBundledRoot := filepath.Join(appRoot, "bundled")
	foundBundledRoot := false
	for _, root := range roots {
		if root == wantBundledRoot {
			foundBundledRoot = true
			break
		}
	}
	if !foundBundledRoot {
		t.Fatalf("RuntimeResourceRoots missing %q: %#v", wantBundledRoot, roots)
	}
}

func TestLoadDesktopSettingsDefaultsWhenMissing(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	got, err := LoadDesktopSettings()
	if err != nil {
		t.Fatalf("LoadDesktopSettings err = %v", err)
	}
	if !got.AutoStartBackend {
		t.Fatalf("LoadDesktopSettings default autoStartBackend = false, want true")
	}
	if got.DeveloperEnvironment.Enabled {
		t.Fatalf("LoadDesktopSettings default developer enabled = true, want false")
	}
	if got.DeveloperEnvironment.BaseURL != "http://10.0.2.2:18787" {
		t.Fatalf("LoadDesktopSettings default developer base url = %q", got.DeveloperEnvironment.BaseURL)
	}
}

func TestSaveDesktopSettingsPersistsValue(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	if err := SaveDesktopSettings(DesktopSettings{
		AutoStartBackend: false,
		DeveloperEnvironment: DeveloperEnvironmentSettings{
			Enabled:              true,
			Mode:                 "workspace",
			RepoPath:             "/tmp/openwatcher",
			BaseURL:              "http://10.0.2.2:18787",
			DeviceName:           "dev-watch",
			HostAlias:            "10.0.2.2",
			ManagedTunnelEnabled: true,
		},
	}); err != nil {
		t.Fatalf("SaveDesktopSettings err = %v", err)
	}

	got, err := LoadDesktopSettings()
	if err != nil {
		t.Fatalf("LoadDesktopSettings err = %v", err)
	}
	if got.AutoStartBackend {
		t.Fatalf("LoadDesktopSettings autoStartBackend = true, want false")
	}
	if !got.DeveloperEnvironment.Enabled {
		t.Fatalf("LoadDesktopSettings developer enabled = false, want true")
	}
	if !got.DeveloperEnvironment.ManagedTunnelEnabled {
		t.Fatalf("LoadDesktopSettings developer managed tunnel = false, want true")
	}

	path, err := desktopSettingsPath()
	if err != nil {
		t.Fatalf("desktopSettingsPath err = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile err = %v", err)
	}
	var decoded DesktopSettings
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal err = %v", err)
	}
	if decoded.AutoStartBackend {
		t.Fatalf("saved autoStartBackend = true, want false")
	}
	if decoded.DeveloperEnvironment.DeviceName != "dev-watch" {
		t.Fatalf("saved developer device name = %q, want dev-watch", decoded.DeveloperEnvironment.DeviceName)
	}
}
