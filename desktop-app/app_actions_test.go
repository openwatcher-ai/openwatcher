package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/settings"
	"openwatcher/desktop-app/internal/tunnel"
	rootconfig "openwatcher/internal/config"
)

func TestBuildBootstrapPayload(t *testing.T) {
	payload, err := buildBootstrapPayload(
		backend.StartConfig{
			PublicBaseURL: "http://192.168.1.12:8787",
		},
		"watch",
		[]BootstrapEndpointRequest{
			{ID: "lan", Label: "局域网", URL: "http://192.168.1.12:8787", Priority: 0},
			{ID: "public", Label: "公网", URL: "https://watch.example.com", Priority: 1},
		},
	)
	if err != nil {
		t.Fatalf("buildBootstrapPayload err = %v", err)
	}
	if payload.APIBase != "http://192.168.1.12:8787" {
		t.Fatalf("APIBase = %q", payload.APIBase)
	}
	if payload.DeviceName != "watch" {
		t.Fatalf("DeviceName = %q", payload.DeviceName)
	}
	if !strings.Contains(payload.BootstrapURI, "endpoints=") {
		t.Fatalf("BootstrapURI should contain endpoints payload: %q", payload.BootstrapURI)
	}
	if payload.TokenFingerprint == "" {
		t.Fatalf("TokenFingerprint should not be empty")
	}
	if token := extractTokenFromBootstrap(payload.BootstrapURI); token == "" {
		t.Fatalf("extractTokenFromBootstrap returned empty token")
	}
}

func TestBuildDevBootstrapPayload(t *testing.T) {
	payload, err := buildDevBootstrapPayload(DevBootstrapRequest{
		BaseURL:    "http://192.168.1.22:8787",
		DeviceName: "watch-dev",
	})
	if err != nil {
		t.Fatalf("buildDevBootstrapPayload err = %v", err)
	}
	if payload.APIBase != "http://192.168.1.22:8787" {
		t.Fatalf("APIBase = %q", payload.APIBase)
	}
	if !strings.Contains(payload.BootstrapURI, "openwatcher://dev-bootstrap?") {
		t.Fatalf("BootstrapURI = %q", payload.BootstrapURI)
	}
	if !strings.Contains(payload.BootstrapURI, "source=desktop-dev") {
		t.Fatalf("BootstrapURI missing dev source: %q", payload.BootstrapURI)
	}
}

func TestNormalizeBootstrapEndpointsFallback(t *testing.T) {
	endpoints, err := normalizeBootstrapEndpoints(nil, "https://watch.example.com/")
	if err != nil {
		t.Fatalf("normalizeBootstrapEndpoints err = %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 fallback endpoint, got %d", len(endpoints))
	}
	if endpoints[0].URL != "https://watch.example.com" {
		t.Fatalf("fallback endpoint url = %q", endpoints[0].URL)
	}
}

func TestExtractTokenFromBootstrapInvalid(t *testing.T) {
	if token := extractTokenFromBootstrap("not-a-url"); token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}

func TestValidateBackendRequestMode(t *testing.T) {
	if err := validateBackendRequestMode(BackendRequest{Mode: "public", CustomURL: "https://demo.example.com"}); err != nil {
		t.Fatalf("unexpected public url validation error: %v", err)
	}
	if err := validateBackendRequestMode(BackendRequest{Mode: "public", CustomURL: "http://demo.example.com"}); err == nil {
		t.Fatalf("expected https validation error")
	}
	if err := validateBackendRequestMode(BackendRequest{Mode: "managed-beta"}); err != nil {
		t.Fatalf("unexpected managed tunnel validation error: %v", err)
	}
}

func TestValidateRequestRequiresRedeemedTunnelBinding(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)
	app := NewApp()
	if err := app.validateRequest(BackendRequest{Mode: "tunnel"}); err == nil {
		t.Fatalf("expected missing binding validation error")
	}

	configDir, err := settings.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir err = %v", err)
	}
	store := tunnel.NewStore(configDir)
	if _, err := store.EnsureIdentity(); err != nil {
		t.Fatalf("EnsureIdentity err = %v", err)
	}
	if err := store.SaveBinding(tunnel.Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
		TokenVersion:  2,
		RedeemedAt:    "2026-06-08T12:00:00Z",
	}, tunnel.RedeemResponse{TunnelToken: "secret-token"}); err != nil {
		t.Fatalf("SaveBinding err = %v", err)
	}

	app = NewApp()
	app.tunnelManager = tunnel.NewManager(settings.AppRoot(), nil, logging.NewRedactor())
	if err := app.validateRequest(BackendRequest{Mode: "tunnel"}); err != nil {
		t.Fatalf("unexpected managed tunnel validation error: %v", err)
	}
}

func TestStartConfigFromRequestManagedTunnelUsesFixedOriginPort(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	app := NewApp()

	configDir, err := settings.ConfigDir()
	if err != nil {
		t.Fatalf("ConfigDir err = %v", err)
	}
	store := tunnel.NewStore(configDir)
	if _, err := store.EnsureIdentity(); err != nil {
		t.Fatalf("EnsureIdentity err = %v", err)
	}
	if err := store.SaveBinding(tunnel.Binding{
		PublicBaseURL: "https://ow-demo.openwatcher.ai",
		TunnelID:      "cf_tunnel_demo",
		TokenVersion:  2,
		RedeemedAt:    "2026-06-08T12:00:00Z",
	}, tunnel.RedeemResponse{TunnelToken: "secret-token"}); err != nil {
		t.Fatalf("SaveBinding err = %v", err)
	}

	cfg := app.startConfigFromRequest(BackendRequest{
		Mode: "tunnel",
		Port: "18787",
	})
	host, port, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		t.Fatalf("SplitHostPort err = %v", err)
	}
	if host != "127.0.0.1" || port == "" {
		t.Fatalf("managed tunnel listen = %q", cfg.Listen)
	}
	if cfg.PublicBaseURL != "https://ow-demo.openwatcher.ai" {
		t.Fatalf("managed tunnel public base = %q", cfg.PublicBaseURL)
	}
}

func TestBackendStartConfigUsesLANRequestDefault(t *testing.T) {
	app := NewApp()
	got := app.backendStartConfig()
	want := app.startConfigFromRequest(BackendRequest{Mode: "lan"})
	if got.Listen != want.Listen || got.PublicBaseURL != want.PublicBaseURL {
		t.Fatalf("backendStartConfig = %#v, want LAN default %#v", got, want)
	}
}

func TestStartConfigFromRequestLANListensOnAllIPv4(t *testing.T) {
	app := NewApp()
	cfg := app.startConfigFromRequest(BackendRequest{
		Mode:       "lan",
		SelectedIP: "192.168.31.187",
		Port:       "8789",
		BindAll:    false,
	})
	if cfg.Listen != "0.0.0.0:8789" {
		t.Fatalf("LAN listen = %q, want 0.0.0.0:8789", cfg.Listen)
	}
	if cfg.PublicBaseURL != "http://192.168.31.187:8789" {
		t.Fatalf("LAN public base = %q", cfg.PublicBaseURL)
	}
}

func TestResolveBootstrapEndpointsUsesRuntimeURLs(t *testing.T) {
	app := NewApp()
	cfg := backend.StartConfig{
		Listen:        "127.0.0.1:8788",
		PublicBaseURL: "http://10.0.2.2:8788",
	}
	resolved := app.resolveBootstrapEndpoints(BackendRequest{
		CustomURL: "https://public.example.com",
		Endpoints: []BootstrapEndpointRequest{
			{ID: "lan", Label: "局域网", URL: "http://10.0.2.2:8787"},
			{ID: "public", Label: "公网", URL: "https://stale.example.com"},
		},
	}, cfg)
	if len(resolved) != 2 {
		t.Fatalf("resolved endpoints len = %d", len(resolved))
	}
	if resolved[0].URL != "http://10.0.2.2:8788" {
		t.Fatalf("resolved lan url = %q", resolved[0].URL)
	}
	if resolved[1].URL != "https://public.example.com" {
		t.Fatalf("resolved public url = %q", resolved[1].URL)
	}
}

func TestRewriteConfiguredPublicBaseURLUsesActualLoopbackPort(t *testing.T) {
	if got := rewriteConfiguredPublicBaseURL("http://10.0.2.2:8787", "127.0.0.1:8789"); got != "http://10.0.2.2:8789" {
		t.Fatalf("rewriteConfiguredPublicBaseURL = %q", got)
	}
	if got := rewriteConfiguredPublicBaseURL("https://ow-demo.openwatcher.ai", "127.0.0.1:8789"); got != "https://ow-demo.openwatcher.ai" {
		t.Fatalf("https public base should stay unchanged, got %q", got)
	}
}

func TestGetDeveloperEnvironmentSnapshotDoesNotStartWorkspace(t *testing.T) {
	app := NewApp()
	repoPath := createWorkspaceRepoForAppActionsTest(t, `#!/usr/bin/env bash
set -euo pipefail
echo started > "$PWD/start-count.txt"
sleep 30
`)

	snapshot := app.GetDeveloperEnvironmentSnapshot(DeveloperEnvironmentRequest{
		Enabled:  true,
		Mode:     "workspace",
		RepoPath: repoPath,
		BaseURL:  testDeveloperBaseURL(t),
	})

	if snapshot.Status.Running {
		t.Fatalf("snapshot read should not start workspace process")
	}
	if _, err := os.Stat(filepath.Join(repoPath, "start-count.txt")); !os.IsNotExist(err) {
		t.Fatalf("snapshot read should not execute workspace script")
	}
}

func TestSetPairingConfig(t *testing.T) {
	cfg := rootconfig.Config{}
	now := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	setPairingConfig(&cfg, rootconfig.PairingSlotBeta, "abcdefghijklmnopqrstuvwxyz012345", "watch", now)
	if cfg.TokenHash == "" {
		t.Fatalf("expected token hash to be written")
	}
	if cfg.TokenHash == "abcdefghijklmnopqrstuvwxyz012345" {
		t.Fatalf("token should be hashed")
	}
	if cfg.DeviceName != "watch" {
		t.Fatalf("device name = %q", cfg.DeviceName)
	}
	if cfg.PairedAt != now.Format(time.RFC3339) {
		t.Fatalf("paired at = %q", cfg.PairedAt)
	}
	if cfg.TokenHashForSlot(rootconfig.PairingSlotBeta) == "" {
		t.Fatalf("expected beta slot token hash to be written")
	}
}

func createWorkspaceRepoForAppActionsTest(t *testing.T, script string) string {
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

func testDeveloperBaseURL(t *testing.T) string {
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
	return "http://10.0.2.2:" + portText
}

func TestClearPairingConfigClearsOnlyRequestedSlot(t *testing.T) {
	cfg := rootconfig.Config{
		TokenHash:  "beta-hash",
		DeviceName: "beta-watch",
		PairedAt:   "2026-06-09T00:00:00Z",
		DevPairing: &rootconfig.PairingBinding{
			TokenHash:  "dev-hash",
			DeviceName: "dev-watch",
			PairedAt:   "2026-06-09T01:00:00Z",
		},
	}
	clearPairingConfig(&cfg, rootconfig.PairingSlotBeta)
	if cfg.TokenHash != "" || cfg.DeviceName != "" || cfg.PairedAt != "" {
		t.Fatalf("beta pairing config not cleared: %#v", cfg)
	}
	if got := cfg.TokenHashForSlot(rootconfig.PairingSlotDev); got != "dev-hash" {
		t.Fatalf("dev pairing should stay untouched, got %q", got)
	}

	cfg = rootconfig.Config{
		TokenHash:  "beta-hash",
		DeviceName: "beta-watch",
		PairedAt:   "2026-06-09T00:00:00Z",
		DevPairing: &rootconfig.PairingBinding{
			TokenHash:  "dev-hash",
			DeviceName: "dev-watch",
			PairedAt:   "2026-06-09T01:00:00Z",
		},
	}
	clearPairingConfig(&cfg, rootconfig.PairingSlotDev)
	if got := cfg.TokenHashForSlot(rootconfig.PairingSlotBeta); got != "beta-hash" {
		t.Fatalf("beta pairing should stay untouched, got %q", got)
	}
	if got := cfg.TokenHashForSlot(rootconfig.PairingSlotDev); got != "" {
		t.Fatalf("dev pairing should be cleared, got %q", got)
	}
}

func TestConstrainedHealthStatusRejectsInvalidPublicModes(t *testing.T) {
	cfg := backend.StartConfig{Listen: "127.0.0.1:8787", PublicBaseURL: "https://demo.example.com"}

	noAuthStatus := constrainedHealthStatus(BackendRequest{Mode: "public", CustomURL: "https://demo.example.com"}, cfg, backend.HealthStatus{
		OK:       true,
		HTTPCode: 200,
		Build:    backend.BuildInfo{Version: "2026.06.07.1"},
		Config: backend.RuntimeInfo{
			Listen:        "127.0.0.1:8787",
			PublicBaseURL: "https://demo.example.com",
			NoAuth:        true,
		},
	})
	if noAuthStatus.OK || noAuthStatus.Message == "" {
		t.Fatalf("expected no-auth rejection, got %#v", noAuthStatus)
	}

	notOpenWatcher := constrainedHealthStatus(BackendRequest{Mode: "public", CustomURL: "https://demo.example.com"}, cfg, backend.HealthStatus{
		OK:       true,
		HTTPCode: 200,
	})
	if notOpenWatcher.OK || notOpenWatcher.Message == "" {
		t.Fatalf("expected identity rejection, got %#v", notOpenWatcher)
	}
}

func TestProbeOpenWatcherHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"build":{"version":"2026.06.07.1","commit":"dev","builtAt":"now"},"config":{"listen":"127.0.0.1:8787","publicBaseUrl":"https://demo.example.com","paired":true,"noAuth":false},"codex":{"homeDetected":true,"authDetected":true,"sessionsDetected":true}}`))
	}))
	defer server.Close()

	status, err := probeOpenWatcherHealth(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("probeOpenWatcherHealth err = %v", err)
	}
	if !status.OK {
		t.Fatalf("expected health ok, got %#v", status)
	}
	if status.Endpoint != server.URL+"/healthz" {
		t.Fatalf("unexpected endpoint %q", status.Endpoint)
	}
}

func TestSubmitRemoteWatchBootstrapSubmitsEvenWhenHealthCheckFails(t *testing.T) {
	var submitted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"ok":false,"message":"not ready"}`))
		case "/v1/watch-bootstrap/config":
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Fatalf("decode submit request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"bootstrapCode":"ABCD2345","status":"ready","config":{"environment":"dev","apiBase":"` + serverURLForResponse(r) + `","source":"desktop-remote-bootstrap","configuredAt":"2026-06-12T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("OPENWATCHER_MANAGED_TUNNEL_API_BASE", server.URL)

	result, err := (&App{}).SubmitRemoteWatchBootstrap(RemoteWatchBootstrapRequest{
		BootstrapCode: " ABCD2345 ",
		Environment:   "dev",
		APIBase:       server.URL + "/",
	})
	if err != nil {
		t.Fatalf("SubmitRemoteWatchBootstrap err = %v", err)
	}
	if submitted["bootstrapCode"] != "ABCD2345" {
		t.Fatalf("bootstrap code not normalized in request: %#v", submitted)
	}
	if _, ok := submitted["deviceToken"]; ok {
		t.Fatalf("deviceToken must not be submitted: %#v", submitted)
	}
	if result.Health.OK {
		t.Fatalf("expected failed health check to be reported")
	}
	if !strings.Contains(result.Message, "未通过 /healthz 检查") {
		t.Fatalf("unexpected result message %q", result.Message)
	}
}

func TestSubmitRemoteWatchBootstrapRedeemsTunnelCodeBeforeSubmitting(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)
	var redeemed bool
	var submitted map[string]any
	var serverURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tunnel/redeem":
			redeemed = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"publicBaseUrl":"` + serverURL + `","tunnelToken":"secret-token","tunnelId":"cf_tunnel_demo","tokenVersion":2,"issuedAt":"2026-06-12T00:00:00Z"}`))
		case "/healthz":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"build":{"version":"2026.06.12.1"},"config":{"listen":"127.0.0.1:8787","publicBaseUrl":"` + serverURL + `","noAuth":false}}`))
		case "/v1/watch-bootstrap/config":
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Fatalf("decode submit request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"bootstrapCode":"ABCD2345","status":"ready","config":{"environment":"beta","apiBase":"` + serverURL + `","source":"desktop-remote-bootstrap","configuredAt":"2026-06-12T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	t.Setenv("OPENWATCHER_MANAGED_TUNNEL_API_BASE", server.URL)

	app := NewApp()
	result, err := app.SubmitRemoteWatchBootstrap(RemoteWatchBootstrapRequest{
		BootstrapCode: "ABCD2345",
		Environment:   "beta",
		TunnelCode:    "OW-TEST-1234",
		APIBase:       "https://ignored.example.com",
	})
	if err != nil {
		t.Fatalf("SubmitRemoteWatchBootstrap err = %v", err)
	}
	if !redeemed {
		t.Fatalf("expected tunnel code to be redeemed")
	}
	if submitted["apiBase"] != server.URL {
		t.Fatalf("expected redeemed public URL to be submitted, got %#v", submitted)
	}
	if !result.TunnelRedeemed || result.APIBase != server.URL {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func serverURLForResponse(r *http.Request) string {
	return "http://" + r.Host
}
