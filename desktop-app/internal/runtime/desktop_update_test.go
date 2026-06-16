package runtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStageDesktopUpdateFindsMacAppBundle(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)

	archivePath := filepath.Join(t.TempDir(), "desktop_v0.3.0_macos_arm64.zip")
	writeZipFile(t, archivePath, map[string]string{
		"OpenWatcher.app/Contents/MacOS/openwatcher": "binary",
		"OpenWatcher.app/Contents/Resources/file":    "resource",
	})

	manager := NewManager(t.TempDir(), "0.3.0")
	manager.platform = "darwin-arm64"

	sourceDir, err := manager.stageDesktopUpdate(archivePath)
	if err != nil {
		t.Fatalf("stageDesktopUpdate err = %v", err)
	}
	if filepath.Base(sourceDir) != "OpenWatcher.app" {
		t.Fatalf("sourceDir = %s", sourceDir)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "Contents", "MacOS", "openwatcher")); err != nil {
		t.Fatalf("staged executable missing: %v", err)
	}
}

func TestArchiveEntryPathRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../evil", "root/../../evil", "/tmp/evil"} {
		if relative, skip := archiveEntryPath(name, ""); !skip {
			t.Fatalf("archiveEntryPath(%q) = %q, false", name, relative)
		}
	}
}
