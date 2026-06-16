package contracts

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const UpdateEnv = "UPDATE_OPENWATCHER_CONTRACT_FIXTURES"

func AssertFixture(t testing.TB, name string, data []byte) {
	t.Helper()
	data = withTrailingNewline(data)
	dir := ContractsDir(t)
	path := filepath.Join(dir, name)
	if os.Getenv(UpdateEnv) == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir contract fixture dir: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write contract fixture %s: %v", name, err)
		}
		return
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read contract fixture %s: %v; run scripts/generate-contract-fixtures.sh", name, err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("contract fixture %s is stale; run scripts/generate-contract-fixtures.sh", name)
	}
}

func ContractsDir(t testing.TB) string {
	t.Helper()
	return filepath.Join(RepoRoot(t), "testdata", "contracts")
}

func RepoRoot(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	dir := wd
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "AGENTS.md")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("repo root not found from %s", wd)
		}
		dir = parent
	}
}

func withTrailingNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	return append(data, '\n')
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func RejectPrivateFragments(t testing.TB, name string, data []byte, fragments ...string) {
	t.Helper()
	text := string(data)
	for _, fragment := range fragments {
		if strings.TrimSpace(fragment) == "" {
			continue
		}
		if strings.Contains(text, fragment) {
			t.Fatalf("contract fixture %s contains private fragment %q", name, fragment)
		}
	}
}
