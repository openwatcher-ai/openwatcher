package tunnel

import (
	"os/exec"
	"runtime"
	"slices"
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
