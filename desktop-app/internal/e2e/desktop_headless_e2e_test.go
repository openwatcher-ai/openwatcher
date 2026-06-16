package e2e

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/adb"
	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/installer"
	"openwatcher/desktop-app/internal/logging"
	desktopruntime "openwatcher/desktop-app/internal/runtime"
	"openwatcher/testsupport/fakeadb"
	"openwatcher/testsupport/fakeruntime"
	"openwatcher/testsupport/fakesidecar"
)

func TestMain(m *testing.M) {
	if fakeadb.MaybeRunProcess() {
		return
	}
	if fakesidecar.MaybeRunProcess() {
		return
	}
	os.Exit(m.Run())
}

func TestDesktopHeadlessSuccess(t *testing.T) {
	h := newHarness(t, fakeadb.State{Connected: false}, fakesidecar.State{})
	ctx := context.Background()

	cfg := backend.StartConfig{
		ConfigPath:    filepath.Join(h.configRoot, "openwatcher.json"),
		Listen:        freeLoopbackListen(t),
		PublicBaseURL: "",
		PairingSlot:   "beta",
	}
	cfg.PublicBaseURL = "http://" + cfg.Listen

	if err := h.backend.StartBackend(ctx, cfg); err != nil {
		t.Fatalf("StartBackend err = %v", err)
	}
	t.Cleanup(func() {
		_ = h.backend.StopBackend(context.Background())
	})
	health := waitForHealth(t, h.backend, cfg)
	if !health.OK {
		t.Fatalf("health not ok: %+v", health)
	}
	if health.Config.PublicBaseURL != cfg.PublicBaseURL {
		t.Fatalf("health publicBaseUrl = %q, want %q", health.Config.PublicBaseURL, cfg.PublicBaseURL)
	}
	if health.Config.PairingSlot != "beta" {
		t.Fatalf("health pairing slot = %q", health.Config.PairingSlot)
	}

	pairStatus := h.installer.PairAndConnect(ctx, installer.PairRequest{
		PairIP:      "192.168.1.33",
		PairPort:    "37123",
		PairingCode: "123456",
		ConnectIP:   "192.168.1.33",
		ConnectPort: "40221",
	})
	if pairStatus.Phase != installer.PhaseDone || pairStatus.SelectedSerial == "" {
		t.Fatalf("PairAndConnect status = %+v", pairStatus)
	}

	installStatus := h.installer.InstallSelected(ctx)
	if installStatus.Phase != installer.PhaseDone {
		t.Fatalf("InstallSelected status = %+v", installStatus)
	}
	if !strings.HasSuffix(installStatus.APK.Path, filepath.Join("watch-apk", "openwatcher-watch-release.apk")) {
		t.Fatalf("installer should use runtime release apk, got %+v", installStatus.APK)
	}

	launchStatus := h.installer.LaunchSelected(ctx)
	if launchStatus.Phase != installer.PhaseDone {
		t.Fatalf("LaunchSelected status = %+v", launchStatus)
	}

	bootstrapURI := buildTestBootstrapURI(t, cfg.PublicBaseURL, "Headless Watch")
	assertBootstrapURIShape(t, bootstrapURI, cfg.PublicBaseURL, "Headless Watch")
	bootstrapStatus := h.installer.StartBootstrap(ctx, bootstrapURI)
	if bootstrapStatus.Phase != installer.PhaseDone {
		t.Fatalf("StartBootstrap status = %+v", bootstrapStatus)
	}
	for _, line := range bootstrapStatus.Logs {
		if strings.Contains(line.Message, "test-token-0123456789abcdef0123456789") {
			t.Fatalf("installer log leaked device token: %+v", line)
		}
	}

	verifyStatus := h.installer.VerifySelected(ctx)
	if verifyStatus.Phase != installer.PhaseDone {
		t.Fatalf("VerifySelected status = %+v", verifyStatus)
	}

	adbState := fakeadb.ReadState(t, fakeadb.StatePath(h.runtime.ADBBinaryPath))
	if !adbState.Installed {
		t.Fatalf("fake adb state should record installed package: %+v", adbState)
	}
	if adbState.LastDeepLink != bootstrapURI {
		t.Fatalf("deep link = %q, want %q", adbState.LastDeepLink, bootstrapURI)
	}

	adbCommands := fakeadb.ReadCommands(t, fakeadb.CommandsPath(h.runtime.ADBBinaryPath))
	assertADBOperations(t, adbCommands, []string{
		"devices -l",
		"pair 192.168.1.33:37123",
		"connect 192.168.1.33:40221",
		"install -r " + h.runtime.WatchAPKPath,
		"shell monkey",
		"shell am start",
		"shell pm path",
		"shell dumpsys package",
	})

	sidecarCommands := fakesidecar.ReadCommands(t, fakesidecar.CommandsPath(h.sidecarBinary))
	if len(sidecarCommands) != 1 {
		t.Fatalf("sidecar command count = %d, commands=%+v", len(sidecarCommands), sidecarCommands)
	}
	if sidecarCommands[0].Listen != cfg.Listen || sidecarCommands[0].PublicBaseURL != cfg.PublicBaseURL {
		t.Fatalf("sidecar command = %+v, want listen=%q public=%q", sidecarCommands[0], cfg.Listen, cfg.PublicBaseURL)
	}
}

func TestDesktopHeadlessSidecarHealthFailure(t *testing.T) {
	h := newHarness(t, fakeadb.State{Connected: true}, fakesidecar.State{HealthHTTPCode: 500})
	cfg := backend.StartConfig{
		ConfigPath:  filepath.Join(h.configRoot, "openwatcher.json"),
		Listen:      freeLoopbackListen(t),
		PairingSlot: "beta",
	}
	cfg.PublicBaseURL = "http://" + cfg.Listen
	if err := h.backend.StartBackend(context.Background(), cfg); err != nil {
		t.Fatalf("StartBackend err = %v", err)
	}
	t.Cleanup(func() {
		_ = h.backend.StopBackend(context.Background())
	})

	health := waitForHealth(t, h.backend, cfg)
	if health.OK {
		t.Fatalf("expected health failure, got %+v", health)
	}
	if health.HTTPCode != 500 {
		t.Fatalf("health code = %d, want 500: %+v", health.HTTPCode, health)
	}
}

func TestDesktopHeadlessADBInstallFailure(t *testing.T) {
	h := newHarness(t, fakeadb.State{Connected: true, FailInstall: true}, fakesidecar.State{})
	h.installer.SelectDevice("192.168.1.33:40221")

	status := h.installer.InstallSelected(context.Background())
	if status.Phase != installer.PhaseTroubleshoot {
		t.Fatalf("expected troubleshoot phase, got %+v", status)
	}
	if !strings.Contains(status.Message, "已有更高版本") {
		t.Fatalf("install failure message = %q", status.Message)
	}
	commands := fakeadb.ReadCommands(t, fakeadb.CommandsPath(h.runtime.ADBBinaryPath))
	assertADBOperations(t, commands, []string{"install -r " + h.runtime.WatchAPKPath})
}

type harness struct {
	configRoot    string
	appRoot       string
	runtime       *fakeruntime.Runtime
	sidecarBinary string
	backend       *backend.Manager
	installer     *installer.Manager
}

func newHarness(t *testing.T, adbState fakeadb.State, sidecarState fakesidecar.State) harness {
	t.Helper()
	configRoot := t.TempDir()
	t.Setenv("HOME", configRoot)
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("OPENWATCHER_CONFIG", filepath.Join(configRoot, "openwatcher", "config.json"))

	appRoot := filepath.Join(t.TempDir(), "desktop-app")
	if err := os.MkdirAll(appRoot, 0o755); err != nil {
		t.Fatalf("mkdir app root: %v", err)
	}

	fakeRuntime := fakeruntime.Start(t, appRoot)
	runtimeManager := desktopruntime.NewManager(appRoot, "0.2.0")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := runtimeManager.EnsureInstaller(ctx); err != nil {
		t.Fatalf("EnsureInstaller err = %v", err)
	}
	fakeadb.WriteState(t, fakeadb.StatePath(fakeRuntime.ADBBinaryPath), adbState)

	sidecarBinary := filepath.Join(
		appRoot,
		"bundled",
		"openwatcher",
		runtime.GOOS+"-"+runtime.GOARCH,
		fakesidecar.BinaryName(),
	)
	fakesidecar.InstallBinary(t, sidecarBinary)
	fakesidecar.WriteState(t, fakesidecar.StatePath(sidecarBinary), sidecarState)

	redactor := logging.NewRedactor()
	backendManager := backend.NewManager(backend.NewBinaryLocator(appRoot), redactor)
	adbService := adb.NewService(adb.NewBinaryLocator(appRoot), redactor)
	installerManager := installer.NewManager(adbService, installer.NewAPKLocator(appRoot), runtimeManager, redactor)

	return harness{
		configRoot:    configRoot,
		appRoot:       appRoot,
		runtime:       fakeRuntime,
		sidecarBinary: sidecarBinary,
		backend:       backendManager,
		installer:     installerManager,
	}
}

func waitForHealth(t *testing.T, manager *backend.Manager, cfg backend.StartConfig) backend.HealthStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last backend.HealthStatus
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		status, _ := manager.CheckHealthWithConfig(ctx, cfg)
		cancel()
		last = status
		if status.HTTPCode != 0 || status.OK {
			return status
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("sidecar health did not respond in time, last=%+v", last)
	return last
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

func buildTestBootstrapURI(t *testing.T, baseURL string, deviceName string) string {
	t.Helper()
	endpoints := []map[string]any{
		{
			"id":       "lan",
			"label":    "局域网",
			"url":      strings.TrimRight(baseURL, "/"),
			"priority": 0,
		},
	}
	payload, err := json.Marshal(endpoints)
	if err != nil {
		t.Fatalf("marshal endpoints: %v", err)
	}
	values := url.Values{}
	values.Set("endpoints", base64.RawURLEncoding.EncodeToString(payload))
	values.Set("deviceToken", "test-token-0123456789abcdef0123456789")
	values.Set("deviceName", deviceName)
	values.Set("source", "desktop-bootstrap")
	return "openwatcher://bootstrap?" + values.Encode()
}

func assertBootstrapURIShape(t *testing.T, raw string, baseURL string, deviceName string) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse bootstrap uri: %v", err)
	}
	if parsed.Scheme != "openwatcher" || parsed.Host != "bootstrap" {
		t.Fatalf("unexpected bootstrap target: %q", raw)
	}
	values := parsed.Query()
	if values.Get("deviceName") != deviceName {
		t.Fatalf("deviceName = %q", values.Get("deviceName"))
	}
	if values.Get("deviceToken") == "" {
		t.Fatalf("device token missing")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(values.Get("endpoints"))
	if err != nil {
		t.Fatalf("decode endpoints: %v", err)
	}
	var endpoints []map[string]any
	if err := json.Unmarshal(decoded, &endpoints); err != nil {
		t.Fatalf("parse endpoints: %v", err)
	}
	if len(endpoints) != 1 || endpoints[0]["url"] != strings.TrimRight(baseURL, "/") {
		t.Fatalf("endpoints = %+v, want base %q", endpoints, baseURL)
	}
}

func assertADBOperations(t *testing.T, records []fakeadb.CommandRecord, want []string) {
	t.Helper()
	operations := make([]string, 0, len(records))
	for _, record := range records {
		operations = append(operations, record.Operation)
	}
	cursor := 0
	for _, operation := range operations {
		if cursor >= len(want) {
			break
		}
		if operation == want[cursor] {
			cursor++
		}
	}
	if cursor != len(want) {
		t.Fatalf("ADB operations = %#v, missing ordered suffix %#v", operations, want[cursor:])
	}
}
