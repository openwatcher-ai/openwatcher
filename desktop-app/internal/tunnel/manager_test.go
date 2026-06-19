package tunnel

import (
	"context"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"testing"

	"openwatcher/desktop-app/internal/logging"
)

func TestClassifyTunnelErrorRecognizesExpiredToken(t *testing.T) {
	message, expired := classifyTunnelError("exit status 255", []LogLine{
		{Message: "Provided Tunnel token is not valid."},
	})
	if !expired {
		t.Fatalf("expected token expired classification")
	}
	if message != TokenExpiredUserMessage {
		t.Fatalf("message = %q", message)
	}
}

func TestCloudflaredRunArgsWithTokenFileKeepsOriginURL(t *testing.T) {
	store := NewStore(t.TempDir())
	binding := Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
	}
	args, err := cloudflaredRunArgs(store, binding, "secret-token", "http://127.0.0.1:8788")
	if err != nil {
		t.Fatalf("cloudflaredRunArgs err = %v", err)
	}
	if !slices.Contains(args, "--url") {
		t.Fatalf("token-file args must keep origin URL: %#v", args)
	}
	if !slices.Contains(args, "http://127.0.0.1:8788") {
		t.Fatalf("token-file args missing actual origin URL: %#v", args)
	}
	if !slices.Contains(args, "--token-file") {
		t.Fatalf("token-file args missing --token-file: %#v", args)
	}
}

func TestCloudflaredRunArgsWithCredentialsUsesRunnerConfig(t *testing.T) {
	store := NewStore(t.TempDir())
	binding := Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
	}
	if err := store.SaveBinding(binding, RedeemResponse{
		TunnelCredentials: &TunnelCredentials{
			AccountTag:   "account",
			TunnelSecret: "secret",
			TunnelID:     "cf_tunnel_demo",
		},
	}); err != nil {
		t.Fatalf("SaveBinding err = %v", err)
	}
	args, err := cloudflaredRunArgs(store, binding, "", "http://127.0.0.1:8788")
	if err != nil {
		t.Fatalf("cloudflaredRunArgs err = %v", err)
	}
	if slices.Contains(args, "--url") {
		t.Fatalf("credentials args should use local config, got %#v", args)
	}
	if !slices.Contains(args, "--config") || !slices.Contains(args, store.RunnerConfigPath()) {
		t.Fatalf("credentials args missing runner config: %#v", args)
	}
}

func TestStopLockedClearsRunningMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep command is not portable on Windows")
	}
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("start sleep: %v", err)
	}
	manager := &Manager{
		cmd:              cmd,
		runningTunnelID:  "old-tunnel",
		runningOriginURL: "http://127.0.0.1:8787",
		redactor:         logging.NewRedactor(),
	}
	if err := manager.stopLocked("test stop"); err != nil {
		t.Fatalf("stopLocked err = %v", err)
	}
	_ = cmd.Wait()
	if manager.cmd != nil {
		t.Fatalf("cmd was not cleared")
	}
	if manager.runningTunnelID != "" || manager.runningOriginURL != "" {
		t.Fatalf("running metadata was not cleared: %q %q", manager.runningTunnelID, manager.runningOriginURL)
	}
}

func TestPublicHealthNotRunningUsesManagerLabel(t *testing.T) {
	manager := &Manager{
		store:    NewNamedStore(t.TempDir(), "managed-dev-tunnel-test"),
		redactor: logging.NewRedactor(),
		label:    "开发环境托管隧道",
	}
	if err := manager.store.SaveBinding(Binding{
		PublicBaseURL: "https://ow-dev.example.com",
		TunnelID:      "cf_tunnel_demo",
	}, RedeemResponse{TunnelToken: "secret-token"}); err != nil {
		t.Fatalf("SaveBinding err = %v", err)
	}

	status, err := manager.PublicHealth(context.Background())
	if err != nil {
		t.Fatalf("PublicHealth err = %v", err)
	}
	if status.OK {
		t.Fatalf("expected not-running health to fail")
	}
	if !strings.Contains(status.Message, "开发环境托管隧道进程尚未启动") {
		t.Fatalf("health message = %q", status.Message)
	}
	if strings.Contains(status.Message, "后端服务") {
		t.Fatalf("health message should not mention backend service: %q", status.Message)
	}
	logs := manager.GetLogs(10)
	if len(logs) == 0 {
		t.Fatalf("expected health log to be recorded")
	}
	last := logs[len(logs)-1].Message
	if !strings.Contains(last, "开发环境托管隧道健康检查失败") || strings.Contains(last, "后端服务") {
		t.Fatalf("health log = %q", last)
	}
}

func TestStartMissingBindingRecordsLabelledLog(t *testing.T) {
	manager := &Manager{
		store:    NewNamedStore(t.TempDir(), "managed-dev-tunnel-test"),
		redactor: logging.NewRedactor(),
		label:    "开发环境托管隧道",
	}
	if err := manager.Start(context.Background(), "http://127.0.0.1:18787"); err == nil {
		t.Fatalf("expected missing binding error")
	}
	logs := manager.GetLogs(10)
	if len(logs) != 1 {
		t.Fatalf("logs len = %d, want 1", len(logs))
	}
	if !strings.Contains(logs[0].Message, "启动开发环境托管隧道失败") {
		t.Fatalf("start failure log = %q", logs[0].Message)
	}
}
