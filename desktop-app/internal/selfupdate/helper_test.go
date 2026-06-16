package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRunHelperReplacesTargetAndWritesStatus(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	backupRoot := filepath.Join(root, "backups")
	statusPath := filepath.Join(root, "status.json")

	writeFile(t, filepath.Join(source, "openwatcher"), "new")
	writeFile(t, filepath.Join(source, "bundled", "resource.txt"), "resource")
	writeFile(t, filepath.Join(target, "openwatcher"), "old")

	options := HelperOptions{
		SourceDir:  source,
		TargetPath: target,
		LaunchPath: filepath.Join(target, "openwatcher"),
		StatusPath: statusPath,
		BackupRoot: backupRoot,
		Platform:   runtime.GOOS,
		Version:    "0.3.1",
		Artifact:   "OpenWatcher-Desktop-test.zip",
	}
	if err := replaceOnlyForTest(options); err != nil {
		t.Fatalf("replaceOnlyForTest err = %v", err)
	}
	if got := readFile(t, filepath.Join(target, "openwatcher")); got != "new" {
		t.Fatalf("target executable = %q", got)
	}
	if got := readFile(t, filepath.Join(target, "bundled", "resource.txt")); got != "resource" {
		t.Fatalf("target resource = %q", got)
	}
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("ReadDir backupRoot: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("backup entries = %d", len(entries))
	}
	if got := readFile(t, filepath.Join(backupRoot, entries[0].Name(), "openwatcher")); got != "old" {
		t.Fatalf("backup executable = %q", got)
	}
	status := NewStatus("installed", "Desktop 已更新并重新启动", options.Version, options.Artifact, filepath.Join(backupRoot, entries[0].Name()))
	if err := WriteStatus(statusPath, status); err != nil {
		t.Fatalf("WriteStatus err = %v", err)
	}
	readStatus, err := ReadStatus(statusPath)
	if err != nil {
		t.Fatalf("ReadStatus err = %v", err)
	}
	if readStatus.Phase != "installed" || readStatus.Version != "0.3.1" {
		t.Fatalf("status = %+v", readStatus)
	}
}

func replaceOnlyForTest(options HelperOptions) error {
	_, err := replaceWithBackup(options)
	return err
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(data)
}
