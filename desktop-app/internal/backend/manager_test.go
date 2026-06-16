package backend

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/logging"
	"openwatcher/testsupport/fakesidecar"
)

func TestMain(m *testing.M) {
	if fakesidecar.MaybeRunProcess() {
		return
	}
	os.Exit(m.Run())
}

func TestManagerStartBackendRecordsArgsAndLogs(t *testing.T) {
	h := newBackendHarness(t, fakesidecar.State{
		StdoutLines: []string{"stdout token=secret-token"},
		StderrLines: []string{"stderr pairing code=889900"},
	})
	cfg := StartConfig{
		ConfigPath:    filepath.Join(h.configRoot, "openwatcher", "sidecar.json"),
		Listen:        freeLoopbackListen(t),
		PublicBaseURL: "https://public.example/openwatcher/",
		PairingSlot:   "dev",
	}
	t.Cleanup(func() {
		_ = h.manager.StopBackend(context.Background())
	})

	if err := h.manager.StartBackend(context.Background(), cfg); err != nil {
		t.Fatalf("StartBackend err = %v", err)
	}
	health := waitForHealth(t, h.manager, cfg, func(status HealthStatus, err error) bool {
		return err == nil && status.OK
	})
	if health.Config.Listen != cfg.Listen {
		t.Fatalf("health listen = %q, want %q", health.Config.Listen, cfg.Listen)
	}
	if health.Config.PublicBaseURL != "https://public.example/openwatcher" {
		t.Fatalf("health publicBaseUrl = %q", health.Config.PublicBaseURL)
	}
	if health.Config.PairingSlot != "dev" {
		t.Fatalf("health pairingSlot = %q", health.Config.PairingSlot)
	}

	commands := fakesidecar.ReadCommands(t, fakesidecar.CommandsPath(h.sidecarBinary))
	if len(commands) != 1 {
		t.Fatalf("command count = %d, want 1: %+v", len(commands), commands)
	}
	command := commands[0]
	if command.Listen != cfg.Listen || command.PublicBaseURL != "https://public.example/openwatcher" || command.PairingSlot != "dev" {
		t.Fatalf("command record = %+v", command)
	}
	assertArgsContainOrdered(t, command.Args, []string{
		"serve",
		"--config", cfg.ConfigPath,
		"--listen", cfg.Listen,
		"--public-base-url", "https://public.example/openwatcher",
		"--pairing-slot", "dev",
	})

	status := h.manager.DesktopStatus()
	if status.State != "running" || !status.Running {
		t.Fatalf("DesktopStatus = %+v, want running", status)
	}
	if status.ConfigPathLabel != "~/openwatcher/sidecar.json" {
		t.Fatalf("ConfigPathLabel = %q", status.ConfigPathLabel)
	}
	if status.ConfiguredPublicBaseURL != "https://public.example/openwatcher" {
		t.Fatalf("ConfiguredPublicBaseURL = %q", status.ConfiguredPublicBaseURL)
	}

	logs := h.manager.GetBackendLogs(20)
	assertLogContains(t, logs, "fake sidecar listening")
	assertLogContains(t, logs, "stdout token=[REDACTED]")
	assertLogContains(t, logs, "stderr pairing code=[REDACTED]")
	assertLogNotContains(t, logs, "secret-token")
	assertLogNotContains(t, logs, "889900")
}

func TestManagerCheckHealthMalformed(t *testing.T) {
	h := newBackendHarness(t, fakesidecar.State{Malformed: true})
	cfg := StartConfig{Listen: freeLoopbackListen(t)}
	cfg.PublicBaseURL = "http://" + cfg.Listen
	t.Cleanup(func() {
		_ = h.manager.StopBackend(context.Background())
	})

	if err := h.manager.StartBackend(context.Background(), cfg); err != nil {
		t.Fatalf("StartBackend err = %v", err)
	}
	health := waitForHealth(t, h.manager, cfg, func(status HealthStatus, err error) bool {
		return err != nil && status.Message == "healthz 返回无法解析"
	})
	if health.HTTPCode != 200 || health.RawBody != "{not-json" {
		t.Fatalf("malformed health = %+v", health)
	}
	status := h.manager.DesktopStatus()
	if status.LastHealth == nil || status.LastHealth.Message != "healthz 返回无法解析" {
		t.Fatalf("DesktopStatus.LastHealth = %+v", status.LastHealth)
	}
}

func TestManagerCrashClassifiesProcessErrors(t *testing.T) {
	t.Run("codex access token missing", func(t *testing.T) {
		h := newBackendHarness(t, fakesidecar.State{
			Crash:        true,
			CrashMessage: "codex access token missing",
		})
		cfg := StartConfig{Listen: freeLoopbackListen(t)}

		if err := h.manager.StartBackend(context.Background(), cfg); err != nil {
			t.Fatalf("StartBackend err = %v", err)
		}
		status := waitForDesktopStatus(t, h.manager, func(status DesktopStatus) bool {
			return status.State == "error"
		})
		if status.Message != "Codex 当前未登录，缺少 access token。" {
			t.Fatalf("DesktopStatus message = %q", status.Message)
		}
		assertLogContains(t, h.manager.GetBackendLogs(20), "codex access token missing")
		assertLogContains(t, h.manager.GetBackendLogs(20), "backend sidecar exited: Codex 当前未登录")
	})

	t.Run("address already in use", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen occupied port: %v", err)
		}
		defer listener.Close()

		h := newBackendHarness(t, fakesidecar.State{})
		cfg := StartConfig{Listen: listener.Addr().String()}

		if err := h.manager.StartBackend(context.Background(), cfg); err != nil {
			t.Fatalf("StartBackend err = %v", err)
		}
		status := waitForDesktopStatus(t, h.manager, func(status DesktopStatus) bool {
			return status.State == "error"
		})
		if status.Message != "监听地址已被占用，请更换端口或停止占用进程。" {
			t.Fatalf("DesktopStatus message = %q", status.Message)
		}
		assertLogContains(t, h.manager.GetBackendLogs(20), "address already in use")
	})
}

func TestManagerStopAndRestart(t *testing.T) {
	h := newBackendHarness(t, fakesidecar.State{})
	cfg1 := StartConfig{Listen: freeLoopbackListen(t), PairingSlot: "beta"}
	cfg1.PublicBaseURL = "http://" + cfg1.Listen
	cfg2 := StartConfig{Listen: freeLoopbackListen(t), PairingSlot: "dev"}
	cfg2.PublicBaseURL = "http://" + cfg2.Listen
	t.Cleanup(func() {
		_ = h.manager.StopBackend(context.Background())
	})

	if err := h.manager.StartBackend(context.Background(), cfg1); err != nil {
		t.Fatalf("StartBackend cfg1 err = %v", err)
	}
	waitForHealth(t, h.manager, cfg1, func(status HealthStatus, err error) bool {
		return err == nil && status.OK
	})

	if err := h.manager.RestartBackend(context.Background(), cfg2); err != nil {
		t.Fatalf("RestartBackend err = %v", err)
	}
	health := waitForHealth(t, h.manager, cfg2, func(status HealthStatus, err error) bool {
		return err == nil && status.OK
	})
	if health.Config.Listen != cfg2.Listen || health.Config.PairingSlot != "dev" {
		t.Fatalf("health after restart = %+v", health)
	}
	status := h.manager.DesktopStatus()
	if status.State != "running" || status.ConfiguredListen != cfg2.Listen || status.ConfiguredPairingSlot != "dev" {
		t.Fatalf("DesktopStatus after restart = %+v", status)
	}

	commands := waitForCommandCount(t, h.sidecarBinary, 2)
	if commands[0].Listen != cfg1.Listen || commands[1].Listen != cfg2.Listen {
		t.Fatalf("commands after restart = %+v", commands)
	}
	assertLogContains(t, h.manager.GetBackendLogs(50), "backend sidecar stopped")

	if err := h.manager.StopBackend(context.Background()); err != nil {
		t.Fatalf("StopBackend err = %v", err)
	}
	stopped := h.manager.DesktopStatus()
	if stopped.Running || stopped.State == "running" {
		t.Fatalf("DesktopStatus after stop = %+v", stopped)
	}
	assertLogContains(t, h.manager.GetBackendLogs(50), "backend sidecar stopped")
}

type backendHarness struct {
	configRoot    string
	appRoot       string
	sidecarBinary string
	manager       *Manager
}

func newBackendHarness(t *testing.T, sidecarState fakesidecar.State) *backendHarness {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("OPENWATCHER_CONFIG", filepath.Join(configRoot, "openwatcher", "config.json"))

	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	sidecarBinary := filepath.Join(appRoot, "bundled", "openwatcher", platformDir(), fakesidecar.BinaryName())
	fakesidecar.InstallBinary(t, sidecarBinary)
	fakesidecar.WriteState(t, fakesidecar.StatePath(sidecarBinary), sidecarState)

	redactor := logging.NewRedactor()
	return &backendHarness{
		configRoot:    configRoot,
		appRoot:       appRoot,
		sidecarBinary: sidecarBinary,
		manager:       NewManager(NewBinaryLocator(appRoot), redactor),
	}
}

func platformDir() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

func freeLoopbackListen(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func waitForHealth(t *testing.T, manager *Manager, cfg StartConfig, done func(HealthStatus, error) bool) HealthStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last HealthStatus
	var lastErr error
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		status, err := manager.CheckHealthWithConfig(ctx, cfg)
		cancel()
		last = status
		lastErr = err
		if done(status, err) {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("health condition not met, last=%+v err=%v", last, lastErr)
	return last
}

func waitForDesktopStatus(t *testing.T, manager *Manager, done func(DesktopStatus) bool) DesktopStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last DesktopStatus
	for time.Now().Before(deadline) {
		status := manager.DesktopStatus()
		last = status
		if done(status) {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("desktop status condition not met, last=%+v", last)
	return last
}

func waitForCommandCount(t *testing.T, binaryPath string, want int) []fakesidecar.CommandRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last []fakesidecar.CommandRecord
	for time.Now().Before(deadline) {
		commands := fakesidecar.ReadCommands(t, fakesidecar.CommandsPath(binaryPath))
		last = commands
		if len(commands) >= want {
			return commands
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("command count = %d, want >= %d: %+v", len(last), want, last)
	return last
}

func assertArgsContainOrdered(t *testing.T, got []string, want []string) {
	t.Helper()
	cursor := 0
	for _, arg := range got {
		if cursor >= len(want) {
			break
		}
		if arg == want[cursor] {
			cursor++
		}
	}
	if cursor != len(want) {
		t.Fatalf("args = %#v, missing ordered entries %#v", got, want[cursor:])
	}
}

func assertLogContains(t *testing.T, logs []LogLine, want string) {
	t.Helper()
	for _, line := range logs {
		if strings.Contains(line.Message, want) {
			return
		}
	}
	t.Fatalf("logs do not contain %q: %+v", want, logs)
}

func assertLogNotContains(t *testing.T, logs []LogLine, unwanted string) {
	t.Helper()
	for _, line := range logs {
		if strings.Contains(line.Message, unwanted) {
			t.Fatalf("logs contain %q: %+v", unwanted, logs)
		}
	}
}
