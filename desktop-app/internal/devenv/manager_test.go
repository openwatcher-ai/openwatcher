package devenv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/logging"
)

func TestEnsureDoesNotRestartWorkspaceDuringStartupGrace(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail

COUNT_FILE="$PWD/start-count.txt"
count=0
if [[ -f "$COUNT_FILE" ]]; then
  count="$(cat "$COUNT_FILE")"
fi
echo $((count + 1)) > "$COUNT_FILE"
sleep 30
`)
	logPath := filepath.Join(t.TempDir(), "developer-environment.log")
	manager := NewManagerWithLogPath(logging.NewRedactor(), logPath)
	goBinary := createExecutableFile(t, "go", "#!/usr/bin/env bash\nexit 0\n")
	manager.goLookup = func() (string, error) { return goBinary, nil }
	cfg := Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  testDeviceBaseURL(t),
	}
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	manager.Ensure(context.Background(), cfg)
	waitForFile(t, filepath.Join(repoPath, "start-count.txt"))

	manager.Ensure(context.Background(), cfg)
	time.Sleep(200 * time.Millisecond)

	if got := strings.TrimSpace(readFile(t, filepath.Join(repoPath, "start-count.txt"))); got != "1" {
		t.Fatalf("expected workspace to start once during startup grace, got %q", got)
	}
}

func TestObserveDoesNotStartWorkspace(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail
echo started > "$PWD/start-count.txt"
sleep 30
`)
	logPath := filepath.Join(t.TempDir(), "developer-environment.log")
	manager := NewManagerWithLogPath(logging.NewRedactor(), logPath)
	goBinary := createExecutableFile(t, "go", "#!/usr/bin/env bash\nexit 0\n")
	manager.goLookup = func() (string, error) { return goBinary, nil }
	cfg := Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  testDeviceBaseURL(t),
	}

	status := manager.Observe(context.Background(), cfg)
	if status.Running {
		t.Fatalf("observe should not start workspace process")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "start-count.txt")); !os.IsNotExist(err) {
		t.Fatalf("observe should not execute workspace script")
	}
}

func TestEnsureWritesDeveloperLogFileAndRecordsCleanExit(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail
echo "booting"
exit 0
`)
	logPath := filepath.Join(t.TempDir(), "developer-environment.log")
	manager := NewManagerWithLogPath(logging.NewRedactor(), logPath)
	goBinary := createExecutableFile(t, "go", "#!/usr/bin/env bash\nexit 0\n")
	manager.goLookup = func() (string, error) { return goBinary, nil }
	cfg := Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  testDeviceBaseURL(t),
	}

	manager.Ensure(context.Background(), cfg)
	waitForFile(t, logPath)
	waitForLogSubstring(t, logPath, "开发环境退出：进程已结束")

	status := manager.Status()
	if status.LastError != "开发环境进程已退出" {
		t.Fatalf("expected clean exit message, got %q", status.LastError)
	}
	if status.LogFileLabel == "" {
		t.Fatalf("expected log file label to be populated")
	}
}

func TestEnsurePassesResolvedGoBinaryIntoWorkspaceScript(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail
printf '%s' "${OPENWATCHER_GO_BIN:-}" > "$PWD/go-bin.txt"
sleep 30
`)
	logPath := filepath.Join(t.TempDir(), "developer-environment.log")
	manager := NewManagerWithLogPath(logging.NewRedactor(), logPath)
	manager.goLookup = func() (string, error) { return "/tmp/custom-go/bin/go", nil }
	cfg := Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  testDeviceBaseURL(t),
	}
	defer func() {
		_ = manager.Stop(context.Background())
	}()

	manager.Ensure(context.Background(), cfg)
	waitForFile(t, filepath.Join(repoPath, "go-bin.txt"))

	if got := strings.TrimSpace(readFile(t, filepath.Join(repoPath, "go-bin.txt"))); got != "/tmp/custom-go/bin/go" {
		t.Fatalf("expected OPENWATCHER_GO_BIN to be passed through, got %q", got)
	}
}

func TestEnsureReturnsFriendlyErrorWhenGoBinaryMissing(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail
sleep 1
`)
	logPath := filepath.Join(t.TempDir(), "developer-environment.log")
	manager := NewManagerWithLogPath(logging.NewRedactor(), logPath)
	manager.goLookup = func() (string, error) {
		return "", os.ErrNotExist
	}
	cfg := Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  testDeviceBaseURL(t),
	}

	status := manager.Ensure(context.Background(), cfg)
	if !strings.Contains(status.Message, "未找到") || !strings.Contains(status.Message, "Go") {
		t.Fatalf("expected friendly go error, got %q", status.Message)
	}
}

func TestObserveReportsExistingWorkspaceServiceAsRunning(t *testing.T) {
	repoPath := createWorkspaceRepo(t, `#!/usr/bin/env bash
set -euo pipefail
sleep 30
`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	manager := NewManagerWithLogPath(logging.NewRedactor(), filepath.Join(t.TempDir(), "developer-environment.log"))
	status := manager.Observe(context.Background(), Config{
		Enabled:  true,
		Mode:     string(ModeWorkspace),
		RepoPath: repoPath,
		BaseURL:  server.URL,
	})

	if !status.Running {
		t.Fatalf("expected existing workspace service to be reported as running")
	}
	if status.ManagedByDesktop {
		t.Fatalf("expected existing service to be marked as externally managed")
	}
	if !status.ExternallyManaged {
		t.Fatalf("expected externally managed flag to be true")
	}
}

func TestNormalizeLoopbackHealthEndpointForTunnelURLUsesLoopback(t *testing.T) {
	got := normalizeLoopbackHealthEndpoint("https://ow-hx0oft8y.openwatcher.ai")
	want := "http://127.0.0.1:18787/healthz"
	if got != want {
		t.Fatalf("normalizeLoopbackHealthEndpoint() = %q, want %q", got, want)
	}
}

func TestLoopbackListenForBaseURLForTunnelURLUsesLoopback(t *testing.T) {
	got := loopbackListenForBaseURL("https://ow-hx0oft8y.openwatcher.ai")
	want := "127.0.0.1:18787"
	if got != want {
		t.Fatalf("loopbackListenForBaseURL() = %q, want %q", got, want)
	}
}

func createWorkspaceRepo(t *testing.T, script string) string {
	t.Helper()
	repoPath := t.TempDir()
	scriptsDir := filepath.Join(repoPath, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	scriptPath := filepath.Join(scriptsDir, "start-local.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return repoPath
}

func createExecutableFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
	return path
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}

func waitForLogSubstring(t *testing.T, path, substring string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(readFile(t, path), substring) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in %s", substring, path)
}

func testDeviceBaseURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	return fmt.Sprintf("http://10.0.2.2:%s", portText)
}
