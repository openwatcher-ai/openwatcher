package backend

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolvePrefersBundledBinary(t *testing.T) {
	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	binaryName := "openwatcher"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(appRoot, "bundled", "openwatcher", runtime.GOOS+"-"+runtime.GOARCH, binaryName)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	locator := NewBinaryLocator(appRoot)
	got, err := locator.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got.Path != binaryPath {
		t.Fatalf("binary path = %q, want %q", got.Path, binaryPath)
	}
}
