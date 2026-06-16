package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"openwatcher/internal/buildinfo"
	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
	"openwatcher/internal/quota"
	"openwatcher/internal/sessions"
)

func TestMain(m *testing.M) {
	cacheDir, err := os.MkdirTemp("", "openwatcher-server-test-cache-")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("OPENWATCHER_CACHE_DIR", cacheDir)
	code := m.Run()
	_ = os.RemoveAll(cacheDir)
	os.Exit(code)
}

func TestHealthReturnsBuildInfoWithoutToken(t *testing.T) {
	codexHome := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(filepath.Join(codexHome, "sessions"), 0o700); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"tokens":{"access_token":"secret"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	cfg := config.Config{
		Listen:        "127.0.0.1:18787",
		PublicBaseURL: "https://desktop.example.test/",
		TokenHash:     "super-secret-hash",
		CodexHome:     codexHome,
	}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.SetNoAuth(true)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("health status = %d body=%s", res.Code, res.Body.String())
	}

	var payload healthResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode health payload: %v body=%s", err, res.Body.String())
	}
	if !payload.OK {
		t.Fatalf("health ok = false: %#v", payload)
	}
	if payload.Build.Version != buildinfo.Version {
		t.Fatalf("health version = %q want %q", payload.Build.Version, buildinfo.Version)
	}
	if payload.Build.Commit == "" || payload.Build.BuiltAt == "" {
		t.Fatalf("health build info incomplete: %#v", payload.Build)
	}
	if payload.Config.Listen != "127.0.0.1:18787" {
		t.Fatalf("health listen = %q", payload.Config.Listen)
	}
	if payload.Config.PublicBaseURL != "https://desktop.example.test" {
		t.Fatalf("health publicBaseURL = %q", payload.Config.PublicBaseURL)
	}
	if !payload.Config.Paired || !payload.Config.NoAuth {
		t.Fatalf("health config flags = %#v", payload.Config)
	}
	if !payload.Codex.HomeDetected || !payload.Codex.AuthDetected || !payload.Codex.SessionsDetected {
		t.Fatalf("health codex = %#v", payload.Codex)
	}
	for _, forbidden := range []string{"super-secret-hash", "secret", codexHome} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("health leaked %q: %s", forbidden, res.Body.String())
		}
	}
}

func TestStatusRequiresTokenAndReturnsSafePayload(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	attachTestPricing(t, app)
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status without token = %d", res.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status with token = %d body=%s", res.Code, res.Body.String())
	}

	body := res.Body.String()
	for _, forbidden := range []string{token, "account", "cookie", "secret prompt"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"contextPressurePercent":19`) {
		t.Fatalf("response missing session pressure data: %s", body)
	}
	if !strings.Contains(body, `"contextCompactThresholdPercent":92`) {
		t.Fatalf("response missing compact threshold data: %s", body)
	}
	if !strings.Contains(body, `"heatmap24h"`) {
		t.Fatalf("response missing session data: %s", body)
	}
	if !strings.Contains(body, `"dailyUsage"`) {
		t.Fatalf("response missing daily usage data: %s", body)
	}
	if !strings.Contains(body, `"estimatedValueLabel"`) {
		t.Fatalf("response missing daily usage value label: %s", body)
	}
	if strings.Contains(body, `"heatmap24h":{"dailyUsage"`) {
		t.Fatalf("daily usage was nested under heatmap: %s", body)
	}
}

func TestStatusAllowsNoAuthWhenEnabled(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.SetNoAuth(true)
	attachTestPricing(t, app)
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status with no-auth = %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"sessions"`) {
		t.Fatalf("status with no-auth missing sessions: %s", res.Body.String())
	}
}

func TestStatusIncludesDailyTrendOnlyWhenRequested(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	attachTestPricing(t, app)
	app.clock = func() time.Time { return time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC) }

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status without trend = %d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), `"dailyTrend30d"`) {
		t.Fatalf("trend returned without request: %s", res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/status?includeDailyTrend30d=1", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status with trend = %d body=%s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `"dailyTrend30d"`) || !strings.Contains(body, `"endDate":"2026-06-04"`) {
		t.Fatalf("trend missing from requested status: %s", body)
	}
	if !strings.Contains(body, `"estimatedValueLabel"`) {
		t.Fatalf("trend missing estimated value label: %s", body)
	}
}

func TestStatusOmitsQuotaErrorsAndExposesQuotaStatus(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeStaleQuotaWithErrors{}, fakeSessions{})
	attachTestPricing(t, app)
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status with stale quota = %d body=%s", res.Code, res.Body.String())
	}

	var payload statusResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode status payload: %v body=%s", err, res.Body.String())
	}
	if payload.Quota == nil {
		t.Fatalf("quota missing in payload: %s", res.Body.String())
	}
	if payload.Quota.Status != quota.StatusStale {
		t.Fatalf("quota status = %q, want %q", payload.Quota.Status, quota.StatusStale)
	}
	if len(payload.Errors) != 0 {
		t.Fatalf("errors = %#v, want none", payload.Errors)
	}
	if strings.Contains(res.Body.String(), "quota refresh failed") {
		t.Fatalf("quota error leaked into response: %s", res.Body.String())
	}
}

func TestStatusStreamRequiresToken(t *testing.T) {
	cfg := config.Config{TokenHash: pairing.HashToken("0123456789abcdef0123456789abcdef")}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/api/status/stream", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status stream without token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestStatusStreamIncludesDailyTrendOnlyInRequestedInitialSnapshot(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	source := newMutableSessions(fakeSessions{}.mustSnapshot(t))
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, source)
	attachTestPricing(t, app)
	app.statusStreamPollInterval = time.Hour
	app.streamHeartbeatInterval = time.Hour
	app.clock = func() time.Time { return time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC) }

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStatusStream(t, server.URL, token)
	first := readSSEEvent(t, reader, time.Second)
	_ = response.Body.Close()
	if first.Name != "status_snapshot" {
		t.Fatalf("first status stream event = %#v", first)
	}
	if strings.Contains(first.Data, `"dailyTrend30d"`) {
		t.Fatalf("trend returned without stream request: %s", first.Data)
	}

	response, reader = openStatusStreamWithQuery(t, server.URL, token, "includeDailyTrend30d=1")
	defer response.Body.Close()
	first = readSSEEvent(t, reader, time.Second)
	if first.Name != "status_snapshot" {
		t.Fatalf("first status stream event = %#v", first)
	}
	if !strings.Contains(first.Data, `"dailyTrend30d"`) || !strings.Contains(first.Data, `"endDate":"2026-06-04"`) {
		t.Fatalf("trend missing from requested stream snapshot: %s", first.Data)
	}
	if !strings.Contains(first.Data, `"estimatedValueLabel"`) {
		t.Fatalf("trend missing estimated value label: %s", first.Data)
	}
}

func TestStatusStreamSendsInitialSnapshotAndChangedSectionsOnly(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	source := newMutableSessions(fakeSessions{}.mustSnapshot(t))
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, source)
	attachTestPricing(t, app)
	app.statusStreamPollInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond
	var tick int64
	app.clock = func() time.Time {
		return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC).Add(time.Duration(atomic.AddInt64(&tick, 1)) * time.Second)
	}

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStatusStream(t, server.URL, token)
	defer response.Body.Close()

	first := readSSEEvent(t, reader, time.Second)
	if first.Name != "status_snapshot" {
		t.Fatalf("first status stream event = %#v", first)
	}
	if !strings.Contains(first.Data, `"sessions"`) || !strings.Contains(first.Data, `"quota"`) {
		t.Fatalf("initial status snapshot missing sections: %s", first.Data)
	}

	heartbeat := readSSEEvent(t, reader, time.Second)
	if strings.HasPrefix(heartbeat.Name, "status_") {
		t.Fatalf("observedAt-only change produced status section event: %#v", heartbeat)
	}

	next := source.snapshot()
	next.Sessions[0].Title = "changed title"
	source.set(next)

	event := readSSEEventUntil(t, reader, time.Second, "status_sessions")
	if !strings.Contains(event.Data, "changed title") {
		t.Fatalf("status sessions event missing changed title: %s", event.Data)
	}
	for _, forbidden := range []string{`"quota"`, `"heatmap24h"`} {
		if strings.Contains(event.Data, forbidden) {
			t.Fatalf("status sessions event included unrelated section %q: %s", forbidden, event.Data)
		}
	}
}

func TestLocalDebugStatusAllowsLoopbackWithoutToken(t *testing.T) {
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	attachTestPricing(t, app)
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	req := httptest.NewRequest(http.MethodGet, "/debug/status-local", nil)
	req.RemoteAddr = "127.0.0.1:43123"
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("loopback debug status = %d body=%s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"contextWindow":920000`) {
		t.Fatalf("debug status missing contextWindow: %s", res.Body.String())
	}
}

func TestLocalDebugStatusRejectsNonLoopback(t *testing.T) {
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/debug/status-local", nil)
	req.RemoteAddr = "192.168.1.20:43123"
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("non-loopback debug status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestPairPageIsAvailableAndDoesNotEchoToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, true, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/pair?deviceToken="+token+"&deviceName=watch", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("pair page status = %d body=%s", res.Code, res.Body.String())
	}

	body := res.Body.String()
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("pair page content type = %q", contentType)
	}
	if strings.Contains(body, token) {
		t.Fatalf("pair page leaked token in body: %s", body)
	}
	for _, expected := range []string{`/api/pair`, `window.history.replaceState`, `deviceToken`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("pair page missing %q: %s", expected, body)
		}
	}
}

func TestPairPageRejectsNonGet(t *testing.T) {
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, true, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/pair", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("pair page non-get status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestLatestAPKServesNewestReleaseForAllowedDevDevice(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "openwatcher-watch-release-older.apk", "old-release", time.Date(2026, 6, 5, 9, 0, 0, 0, time.UTC))
	writeAPKFixture(t, distDir, "openwatcher-watch-release-newer.apk", "new-release", time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	writeAPKFixture(t, distDir, "openwatcher-watch-debug-latest.apk", "debug-build", time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdef"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "new-release" {
		t.Fatalf("latest apk body = %q", got)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "application/vnd.android.package-archive") {
		t.Fatalf("latest apk content type = %q", contentType)
	}
	if disposition := res.Header().Get("Content-Disposition"); !strings.Contains(disposition, "openwatcher-watch-release-newer.apk") {
		t.Fatalf("latest apk disposition = %q", disposition)
	}
	if got := res.Header().Get("X-OpenWatcher-Apk-Filename"); got != "openwatcher-watch-release-newer.apk" {
		t.Fatalf("latest apk filename header = %q", got)
	}
	if cacheControl := res.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("latest apk cache-control = %q", cacheControl)
	}
}

func TestLatestAPKSupportsHead(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "openwatcher-watch-release-head.apk", "release", time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdea"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodHead, "/file/latest-apk", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk head status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "" {
		t.Fatalf("latest apk head body = %q", got)
	}
	if got := res.Header().Get("X-OpenWatcher-Apk-Size"); got != "7" {
		t.Fatalf("latest apk size header = %q", got)
	}
}

func TestLatestAPKRejectsNonGetOrHead(t *testing.T) {
	token := "0123456789abcdef0123456789abcdeb"
	app := newDevUpdateTestApp(t, t.TempDir(), token)

	req := httptest.NewRequest(http.MethodPost, "/file/latest-apk", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("latest apk post status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestLatestAPKReturnsNotFoundWhenNoReleaseAPKExists(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "openwatcher-watch-debug.apk", "debug", time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	writeAPKFixture(t, distDir, "notes.txt", "notes", time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdec"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("latest apk missing status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestLatestAPKMetadataServesLatestJSONForAllowedDevDevice(t *testing.T) {
	distDir := t.TempDir()
	metadata := `{"versionName":"0.3.0","versionCode":17,"artifact":"openwatcher-watchapp-v0.3.0-release.apk","sha256":"abc123"}`
	writeAPKFixture(t, distDir, "latest-apk.json", metadata, time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	token := "0123456789abcdef0123456789abcded"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk metadata status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != metadata {
		t.Fatalf("latest apk metadata body = %q", got)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("latest apk metadata content type = %q", contentType)
	}
	if cacheControl := res.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("latest apk metadata cache-control = %q", cacheControl)
	}
}

func TestLatestAPKPrefersMetadataArtifactWhenPresent(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "openwatcher-watch-release-newer.apk", "scan-newer", time.Date(2026, 6, 5, 11, 0, 0, 0, time.UTC))
	writeAPKFixture(t, distDir, "openwatcher-watchapp-v0.3.0-update-flow.apk", "metadata-target", time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	writeAPKFixture(t, distDir, "latest-apk.json", `{"artifact":"openwatcher-watchapp-v0.3.0-update-flow.apk"}`, time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdee"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk by metadata status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "metadata-target" {
		t.Fatalf("latest apk by metadata body = %q", got)
	}
	if got := res.Header().Get("X-OpenWatcher-Apk-Filename"); got != "openwatcher-watchapp-v0.3.0-update-flow.apk" {
		t.Fatalf("latest apk by metadata filename header = %q", got)
	}
}

func TestLatestAPKMetadataReturnsNotFoundWhenMissing(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	app := newDevUpdateTestApp(t, t.TempDir(), token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("latest apk metadata missing status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestLatestAPKChangelogServesStructuredJSONForAllowedDevDevice(t *testing.T) {
	distDir := t.TempDir()
	changelog := `[{"versionName":"0.11.0","versionCode":42,"publishedAtUtc":"2026-06-05T20:20:29Z","publishedAtBeijing":"2026-06-06 04:20","summary":"合入会话详情旋钮滚动优化","summaryType":"user"}]`
	writeAPKFixture(t, distDir, "latest-apk-changelog.json", changelog, time.Date(2026, 6, 6, 4, 20, 29, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdea"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk-changelog.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk changelog status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != changelog {
		t.Fatalf("latest apk changelog body = %q", got)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("latest apk changelog content type = %q", contentType)
	}
	if cacheControl := res.Header().Get("Cache-Control"); cacheControl != "no-store" {
		t.Fatalf("latest apk changelog cache-control = %q", cacheControl)
	}
}

func TestLatestAPKChangelogHeadReturnsHeadersWithoutBody(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "latest-apk-changelog.json", "[]", time.Date(2026, 6, 6, 4, 20, 29, 0, time.UTC))
	token := "0123456789abcdef0123456789abcdeb"
	app := newDevUpdateTestApp(t, distDir, token)

	req := httptest.NewRequest(http.MethodHead, "/file/latest-apk-changelog.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("latest apk changelog head status = %d body=%s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "" {
		t.Fatalf("latest apk changelog head body = %q", got)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("latest apk changelog head content type = %q", contentType)
	}
}

func TestLatestAPKChangelogRejectsNonGetOrHead(t *testing.T) {
	token := "0123456789abcdef0123456789abcdec"
	app := newDevUpdateTestApp(t, t.TempDir(), token)

	req := httptest.NewRequest(http.MethodPost, "/file/latest-apk-changelog.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("latest apk changelog post status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestLatestAPKChangelogReturnsNotFoundWhenMissing(t *testing.T) {
	token := "0123456789abcdef0123456789abcded"
	app := newDevUpdateTestApp(t, t.TempDir(), token)

	req := httptest.NewRequest(http.MethodGet, "/file/latest-apk-changelog.json", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("latest apk changelog missing status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestDevUpdateEndpointsRejectUnauthorizedOrUnlistedDevice(t *testing.T) {
	distDir := t.TempDir()
	writeAPKFixture(t, distDir, "latest-apk.json", `{"artifact":"openwatcher-watchapp-v0.3.0-release.apk"}`, time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC))
	allowedToken := "0123456789abcdef0123456789abcdee"
	app := newDevUpdateTestApp(t, distDir, allowedToken)

	req := httptest.NewRequest(http.MethodGet, "/file/dev/latest.json", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("dev latest without token = %d body=%s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/file/dev/latest.json", nil)
	req.Header.Set("X-OpenWatcher-Token", "0123456789abcdef0123456789abcdff")
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("dev latest with wrong token = %d body=%s", res.Code, res.Body.String())
	}

	unlistedToken := "1123456789abcdef0123456789abcdee"
	app = newDevUpdateTestAppWithAllowlist(t, distDir, allowedToken, []string{pairing.HashToken(allowedToken)})
	req = httptest.NewRequest(http.MethodGet, "/file/dev/latest.json", nil)
	req.Header.Set("X-OpenWatcher-Token", unlistedToken)
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("dev latest with unlisted token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestScreenshotUploadRequiresToken(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), ScreenshotUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/screenshots", bytes.NewReader(validPNGFixture()))
	req.Header.Set("Content-Type", "image/png")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("screenshot upload without token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestScreenshotUploadAllowsNoAuthWhenEnabled(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), ScreenshotUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.SetNoAuth(true)

	req := httptest.NewRequest(http.MethodPost, "/api/screenshots", bytes.NewReader(validPNGFixture()))
	req.Header.Set("Content-Type", "image/png")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("screenshot upload with no-auth = %d body=%s", res.Code, res.Body.String())
	}
}

func TestScreenshotUploadStoresPNGWithSafeFilename(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	uploadDir := filepath.Join(t.TempDir(), "screenshots")
	cfg := config.Config{TokenHash: pairing.HashToken(token), ScreenshotUploadDir: uploadDir}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.clock = func() time.Time { return time.Date(2026, 6, 5, 13, 15, 0, 0, time.UTC) }

	body := validPNGFixture()
	req := httptest.NewRequest(http.MethodPost, "/api/screenshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/png; charset=binary")
	req.Header.Set("X-OpenWatcher-Token", token)
	req.Header.Set("X-OpenWatcher-Device-Name", `Xiaomi Watch/../5`)
	req.Header.Set("X-OpenWatcher-App-Version", "0.7.4")
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("screenshot upload status = %d body=%s", res.Code, res.Body.String())
	}

	var payload screenshotUploadResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode screenshot payload: %v body=%s", err, res.Body.String())
	}
	if !payload.OK {
		t.Fatalf("screenshot payload ok = false: %#v", payload)
	}
	if payload.Filename != "watch-20260605T131500Z-xiaomi-watch-5-0.7.4.png" {
		t.Fatalf("screenshot filename = %q", payload.Filename)
	}
	if strings.Contains(payload.Filename, "/") || strings.Contains(payload.Filename, "..") {
		t.Fatalf("screenshot filename was not sanitized: %q", payload.Filename)
	}
	saved, err := os.ReadFile(filepath.Join(uploadDir, payload.Filename))
	if err != nil {
		t.Fatalf("read saved screenshot: %v", err)
	}
	if !bytes.Equal(saved, body) {
		t.Fatalf("saved screenshot bytes = %v, want %v", saved, body)
	}
	info, err := os.Stat(filepath.Join(uploadDir, payload.Filename))
	if err != nil {
		t.Fatalf("stat saved screenshot: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("screenshot mode = %o, want 0600", got)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/screenshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-OpenWatcher-Token", token)
	req.Header.Set("X-OpenWatcher-Device-Name", `Xiaomi Watch/../5`)
	req.Header.Set("X-OpenWatcher-App-Version", "0.7.4")
	res = httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("second screenshot upload status = %d body=%s", res.Code, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode second screenshot payload: %v body=%s", err, res.Body.String())
	}
	if payload.Filename != "watch-20260605T131500Z-xiaomi-watch-5-0.7.4-2.png" {
		t.Fatalf("second screenshot filename = %q", payload.Filename)
	}
	if _, err := os.Stat(filepath.Join(uploadDir, payload.Filename)); err != nil {
		t.Fatalf("stat second screenshot: %v", err)
	}
}

func TestScreenshotUploadRejectsNonPNG(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), ScreenshotUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/screenshots", strings.NewReader("not png"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("screenshot upload non-png status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestScreenshotUploadRejectsOversizedBody(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token), ScreenshotUploadDir: t.TempDir()}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	body := append(validPNGFixture(), bytes.Repeat([]byte{'x'}, screenshotMaxBytes)...)
	req := httptest.NewRequest(http.MethodPost, "/api/screenshots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("screenshot upload oversized status = %d body=%s", res.Code, res.Body.String())
	}
}

func TestPairingSavesOnlyTokenHash(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(configPath, cfg, true, fakeQuota{}, fakeSessions{})
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	body := strings.NewReader(`{"deviceToken":"` + token + `","deviceName":"watch"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pair", body)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("pair status = %d body=%s", res.Code, res.Body.String())
	}

	loaded, _, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.TokenHash == "" || loaded.TokenHash == token {
		t.Fatalf("stored token hash = %q", loaded.TokenHash)
	}
	if !pairing.VerifyTokenHash(token, loaded.TokenHash) {
		t.Fatal("stored hash does not verify")
	}
	if !pairing.VerifyTokenHash(token, loaded.TokenHashForSlot(config.PairingSlotBeta)) {
		t.Fatal("beta slot hash does not verify")
	}
}

func TestPairingSavesDevSlotTokenHash(t *testing.T) {
	token := "1123456789abcdef0123456789abcdef"
	configPath := filepath.Join(t.TempDir(), "config.json")
	cfg := config.Config{}
	cfg.ApplyDefaults()
	app := New(configPath, cfg, true, fakeQuota{}, fakeSessions{})
	app.SetPairingSlot(config.PairingSlotDev)
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }

	body := strings.NewReader(`{"deviceToken":"` + token + `","deviceName":"dev-watch"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/pair", body)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("pair status = %d body=%s", res.Code, res.Body.String())
	}

	loaded, _, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if !pairing.VerifyTokenHash(token, loaded.TokenHashForSlot(config.PairingSlotDev)) {
		t.Fatal("dev slot hash does not verify")
	}
	if loaded.TokenHashForSlot(config.PairingSlotBeta) != "" {
		t.Fatalf("beta slot should stay empty, got %q", loaded.TokenHashForSlot(config.PairingSlotBeta))
	}
}

func TestSessionStreamRequiresToken(t *testing.T) {
	cfg := config.Config{TokenHash: pairing.HashToken("0123456789abcdef0123456789abcdef")}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/thread-1/stream", nil)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("stream without token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestSessionStreamClientEventRequiresToken(t *testing.T) {
	cfg := config.Config{TokenHash: pairing.HashToken("0123456789abcdef0123456789abcdef")}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodPost, "/api/session-stream-events", strings.NewReader(`{"eventType":"disconnect","threadId":"thread-1"}`))
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("session stream client event without token = %d body=%s", res.Code, res.Body.String())
	}
}

func TestSessionStreamClientEventLogsAuthorizedPayload(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	var captured sessionStreamClientEventLog
	app.onSessionStreamClientEvent = func(event sessionStreamClientEventLog) {
		captured = event
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/session-stream-events",
		strings.NewReader(`{"eventType":"disconnect","threadId":"thread-1","deviceName":"Xiaomi Watch 5","appVersion":"0.2.10","reconnectAttempt":2,"reason":"networkerror","detail":"SocketTimeoutException: timeout","statusCode":504,"retryable":true,"connectedMs":12000,"nextRetryDelayMs":5000,"receivedAgentMessage":false}`),
	)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("session stream client event status = %d body=%s", res.Code, res.Body.String())
	}
	if captured.EventType != "disconnect" || captured.ThreadID != "thread-1" {
		t.Fatalf("captured event = %#v", captured)
	}
	if captured.DeviceName != "Xiaomi Watch 5" || captured.AppVersion != "0.2.10" {
		t.Fatalf("captured identity = %#v", captured)
	}
	if captured.ReconnectAttempt != 2 || captured.ConnectedMs != 12000 || captured.NextRetryDelayMs != 5000 {
		t.Fatalf("captured timing = %#v", captured)
	}
	if captured.Retryable == nil || !*captured.Retryable {
		t.Fatalf("captured retryable = %#v", captured.Retryable)
	}
}

func TestSessionStreamReturnsNotFoundForUnknownThread(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/missing-thread/stream", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("unknown stream thread = %d body=%s", res.Code, res.Body.String())
	}
}

func TestSessionStreamSendsInitialLatestAgentMessage(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2"
	rolloutPath := writeServerRollout(t, []string{
		rolloutEventLine(t, "user_message", "用户消息不应出现"),
		rolloutEventLine(t, "agent_message", " 第一条 agent 消息 "),
		rolloutEventLine(t, "token_count", ""),
		rolloutEventLine(t, "agent_message", "最新 agent 消息"),
	})
	app := streamTestApp(t, token, threadID, rolloutPath)

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStream(t, server.URL, token, threadID)
	defer response.Body.Close()

	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("stream content type = %q", contentType)
	}

	event := readSSEEvent(t, reader, time.Second)
	if event.Name != "runtime_state" {
		t.Fatalf("initial runtime event = %#v", event)
	}
	runtimeState := decodeRuntimeState(t, event.Data)
	if runtimeState.ThreadID != threadID || runtimeState.Type != "runtime_state" {
		t.Fatalf("initial runtime state = %#v", runtimeState)
	}

	event = readSSEEvent(t, reader, time.Second)
	if event.Name != "agent_message" || event.ID == "" {
		t.Fatalf("initial event = %#v", event)
	}
	message := decodeAgentMessage(t, event.Data)
	if message.ThreadID != threadID || message.Text != "最新 agent 消息" || message.Truncated {
		t.Fatalf("initial agent message = %#v", message)
	}
	if !strings.Contains(event.Data, `"type":"agent_message"`) || !strings.Contains(event.Data, `"createdAt"`) {
		t.Fatalf("initial data missing fields: %s", event.Data)
	}
}

func TestSessionStreamTailsNewAgentMessageAndFiltersOtherEvents(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1be"
	rolloutPath := writeServerRollout(t, []string{
		rolloutEventLine(t, "token_count", ""),
	})
	app := streamTestApp(t, token, threadID, rolloutPath)
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStream(t, server.URL, token, threadID)
	defer response.Body.Close()

	first := readSSEEvent(t, reader, time.Second)
	if first.Name != "runtime_state" {
		t.Fatalf("first event = %#v", first)
	}

	appendServerRollout(t, rolloutPath, []string{
		rolloutEventLine(t, "user_message", "用户消息不应推送"),
		rolloutEventLine(t, "token_count", ""),
		`{"timestamp":"2026-06-04T01:00:00Z","type":"response_item","payload":{"type":"tool_output","message":"工具输出不应推送"}}`,
		rolloutEventLine(t, "agent_message", "   "),
	})
	heartbeat := readSSEEvent(t, reader, time.Second)
	if heartbeat.Name == "agent_message" {
		t.Fatalf("filtered events produced agent message: %#v", heartbeat)
	}

	appendServerRollout(t, rolloutPath, []string{
		rolloutEventLine(t, "agent_message", "新增 agent 消息"),
	})
	event := readSSEEventUntil(t, reader, time.Second, "agent_message")
	message := decodeAgentMessage(t, event.Data)
	if message.Text != "新增 agent 消息" || message.ThreadID != threadID {
		t.Fatalf("tailed agent message = %#v", message)
	}
	for _, forbidden := range []string{"用户消息不应推送", "工具输出不应推送"} {
		if strings.Contains(event.Data, forbidden) {
			t.Fatalf("stream leaked filtered text %q in %s", forbidden, event.Data)
		}
	}
}

func TestSessionStreamTruncatesLongAgentMessage(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1bf"
	longText := strings.Repeat("界", 4101)
	rolloutPath := writeServerRollout(t, []string{
		rolloutEventLine(t, "agent_message", longText),
	})
	app := streamTestApp(t, token, threadID, rolloutPath)

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStream(t, server.URL, token, threadID)
	defer response.Body.Close()

	event := readSSEEventUntil(t, reader, time.Second, "agent_message")
	message := decodeAgentMessage(t, event.Data)
	if !message.Truncated {
		t.Fatalf("long message was not truncated: %#v", message)
	}
	if got := len([]rune(message.Text)); got != 4096 {
		t.Fatalf("truncated runes = %d, want 4096", got)
	}
}

func TestSessionStreamExitsWhenClientDisconnects(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1c0"
	rolloutPath := writeServerRollout(t, []string{})
	app := streamTestApp(t, token, threadID, rolloutPath)
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond
	exited := make(chan string, 1)
	app.onStreamExit = func(threadID string) {
		exited <- threadID
	}

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStream(t, server.URL, token, threadID)
	first := readSSEEvent(t, reader, time.Second)
	if first.Name != "runtime_state" {
		t.Fatalf("first event = %#v", first)
	}
	_ = response.Body.Close()

	select {
	case got := <-exited:
		if got != threadID {
			t.Fatalf("exit thread id = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not exit after client disconnect")
	}
}

func TestSessionStreamSendsRuntimeStateAndTailsLifecycle(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1d0"
	rolloutPath := writeServerRollout(t, []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:00:01Z","type":"response_item","payload":{"type":"function_call","call_id":"call-1","name":"exec_command","arguments":"不应出现在 SSE"}}`,
	})
	app := streamTestApp(t, token, threadID, rolloutPath)
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 80 * time.Millisecond

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStream(t, server.URL, token, threadID)
	defer response.Body.Close()

	event := readSSEEvent(t, reader, time.Second)
	state := decodeRuntimeState(t, event.Data)
	if event.Name != "runtime_state" || !state.Running || state.Phase != sessions.RuntimePhaseToolRunning {
		t.Fatalf("initial runtime state event=%#v state=%#v", event, state)
	}
	if state.StartedAt != "2026-06-04T01:00:00Z" {
		t.Fatalf("initial runtime startedAt = %q", state.StartedAt)
	}
	if strings.Contains(event.Data, "不应出现在 SSE") {
		t.Fatalf("runtime state leaked tool detail: %s", event.Data)
	}

	appendServerRollout(t, rolloutPath, []string{
		`{"timestamp":"2026-06-04T01:00:02Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"不应出现在 SSE"}}`,
		`{"timestamp":"2026-06-04T01:00:03Z","type":"event_msg","payload":{"type":"agent_message","phase":"final_answer","message":"完成说明"}}`,
		`{"timestamp":"2026-06-04T01:00:04Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
	})
	completed := readSSEEventUntil(t, reader, time.Second, "runtime_state")
	for {
		state = decodeRuntimeState(t, completed.Data)
		if state.Lifecycle == sessions.RuntimeLifecycleCompleted {
			break
		}
		completed = readSSEEventUntil(t, reader, time.Second, "runtime_state")
	}
	if state.Running || state.TurnID != "turn-1" || state.StartedAt != "2026-06-04T01:00:00Z" {
		t.Fatalf("completed runtime state = %#v", state)
	}
	if strings.Contains(completed.Data, "不应出现在 SSE") {
		t.Fatalf("runtime state leaked tool output: %s", completed.Data)
	}
}

func TestSessionStreamIncludeMessagesZeroFiltersAgentMessages(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1d1"
	rolloutPath := writeServerRollout(t, []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		rolloutEventLine(t, "agent_message", "初始消息不应发送"),
	})
	app := streamTestApp(t, token, threadID, rolloutPath)
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openStreamWithQuery(t, server.URL, token, threadID, "includeMessages=0")
	defer response.Body.Close()

	event := readSSEEvent(t, reader, time.Second)
	if event.Name != "runtime_state" {
		t.Fatalf("initial event = %#v", event)
	}
	if strings.Contains(event.Data, "初始消息不应发送") {
		t.Fatalf("runtime state leaked agent message: %s", event.Data)
	}

	appendServerRollout(t, rolloutPath, []string{
		rolloutEventLine(t, "agent_message", "新增消息不应发送"),
	})
	next := readSSEEvent(t, reader, time.Second)
	if next.Name == "agent_message" {
		t.Fatalf("includeMessages=0 produced agent message: %#v", next)
	}
	if strings.Contains(next.Data, "新增消息不应发送") {
		t.Fatalf("includeMessages=0 leaked message text: %s", next.Data)
	}
}

func TestSessionWindowStreamSendsInitialWindow(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadA := "thread-a"
	threadB := "thread-b"
	rolloutA := writeServerRollout(t, []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-a"}}`,
		rolloutEventLine(t, "agent_message", "A 最新消息"),
	})
	rolloutB := writeServerRollout(t, []string{
		rolloutEventLine(t, "agent_message", "B 最新消息"),
	})
	source := fakeSessions{
		rollouts: map[string]string{
			threadA: rolloutA,
			threadB: rolloutB,
		},
		snapshot: &sessions.Snapshot{
			Sessions: []sessions.SessionSnapshot{
				testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 0, 59, 0, 0, time.UTC)),
				testSessionSnapshot(threadB, "Beta", "gpt-5.5-mini", "medium", 2, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
			},
		},
	}
	app := windowStreamTestApp(t, token, source)
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openSessionWindowStream(t, server.URL, token, "limit=5")
	defer response.Body.Close()

	event := readSSEEventUntil(t, reader, time.Second, "sessions_window")
	window := decodeSessionWindowEvent(t, event.Data)
	if len(window.ThreadOrder) != 2 || window.ThreadOrder[0] != threadA || window.ThreadOrder[1] != threadB {
		t.Fatalf("initial window order = %#v", window.ThreadOrder)
	}
	if window.Sessions[0].RuntimeState.ThreadID != threadA || !window.Sessions[0].RuntimeState.Running {
		t.Fatalf("initial runtime state = %#v", window.Sessions[0].RuntimeState)
	}
	if window.Sessions[0].LatestAgentMessage == nil || window.Sessions[0].LatestAgentMessage.Text != "A 最新消息" {
		t.Fatalf("initial latest message = %#v", window.Sessions[0].LatestAgentMessage)
	}
	if window.Sessions[1].LatestAgentMessage == nil || window.Sessions[1].LatestAgentMessage.Text != "B 最新消息" {
		t.Fatalf("second latest message = %#v", window.Sessions[1].LatestAgentMessage)
	}
}

func TestSessionWindowStreamInsertsNewActiveSessionIntoFirstInactiveSlot(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadA := "thread-a"
	threadB := "thread-b"
	threadC := "thread-c"
	rollouts := map[string]string{
		threadA: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-a"}}`,
			rolloutEventLine(t, "agent_message", "A"),
		}),
		threadB: writeServerRollout(t, []string{
			rolloutEventLine(t, "agent_message", "B"),
		}),
		threadC: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:01:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-c"}}`,
			rolloutEventLine(t, "agent_message", "C"),
		}),
	}
	source := newMutableSessionsWithRollouts(
		sessions.Snapshot{
			Sessions: []sessions.SessionSnapshot{
				testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)),
				testSessionSnapshot(threadB, "Beta", "gpt-5.5-mini", "medium", 2, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
			},
		},
		rollouts,
	)
	app := New(filepath.Join(t.TempDir(), "config.json"), config.Config{TokenHash: pairing.HashToken(token)}, false, fakeQuota{}, source)
	app.cfg.ApplyDefaults()
	app.clock = func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) }
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openSessionWindowStream(t, server.URL, token, "")
	defer response.Body.Close()
	first := decodeSessionWindowEvent(t, readSSEEventUntil(t, reader, time.Second, "sessions_window").Data)
	if got := strings.Join(first.ThreadOrder, ","); got != "thread-a,thread-b" {
		t.Fatalf("first window order = %s", got)
	}

	source.set(sessions.Snapshot{
		Sessions: []sessions.SessionSnapshot{
			testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)),
			testSessionSnapshot(threadC, "Gamma", "gpt-5.5", "high", 3, 6, time.Date(2026, 6, 4, 1, 1, 0, 0, time.UTC)),
			testSessionSnapshot(threadB, "Beta", "gpt-5.5-mini", "medium", 2, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
		},
	})

	next := decodeSessionWindowEvent(t, readSSEEventUntil(t, reader, time.Second, "sessions_window").Data)
	if got := strings.Join(next.ThreadOrder, ","); got != "thread-a,thread-c,thread-b" {
		t.Fatalf("updated window order = %s", got)
	}
}

func TestSessionWindowStreamMovesCompletedSessionToInactiveBlock(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadA := "thread-a"
	threadB := "thread-b"
	threadC := "thread-c"
	rollouts := map[string]string{
		threadA: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-a"}}`,
			rolloutEventLine(t, "agent_message", "A"),
		}),
		threadB: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:00:01Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-b"}}`,
			rolloutEventLine(t, "agent_message", "B"),
		}),
		threadC: writeServerRollout(t, []string{
			rolloutEventLine(t, "agent_message", "C"),
		}),
	}
	source := newMutableSessionsWithRollouts(
		sessions.Snapshot{
			Sessions: []sessions.SessionSnapshot{
				testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)),
				testSessionSnapshot(threadB, "Beta", "gpt-5.5", "high", 2, 9, time.Date(2026, 6, 4, 1, 0, 30, 0, time.UTC)),
				testSessionSnapshot(threadC, "Gamma", "gpt-5.5-mini", "medium", 3, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
			},
		},
		rollouts,
	)
	app := New(filepath.Join(t.TempDir(), "config.json"), config.Config{TokenHash: pairing.HashToken(token)}, false, fakeQuota{}, source)
	app.cfg.ApplyDefaults()
	app.clock = func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) }
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	response, reader := openSessionWindowStream(t, server.URL, token, "")
	defer response.Body.Close()
	first := decodeSessionWindowEvent(t, readSSEEventUntil(t, reader, time.Second, "sessions_window").Data)
	if got := strings.Join(first.ThreadOrder, ","); got != "thread-a,thread-b,thread-c" {
		t.Fatalf("first window order = %s", got)
	}
	if first.Sessions[0].RuntimeState.StartedAt != "2026-06-04T01:00:00Z" {
		t.Fatalf("first window startedAt = %q", first.Sessions[0].RuntimeState.StartedAt)
	}

	appendServerRollout(t, rollouts[threadA], []string{
		`{"timestamp":"2026-06-04T01:00:02Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-a"}}`,
	})

	runtimeEvent := readSSEEventUntil(t, reader, time.Second, "session_runtime_state")
	runtimeWrapper := decodeSessionWindowRuntimeStateEvent(t, runtimeEvent.Data)
	if runtimeWrapper.RuntimeState.ThreadID != threadA || runtimeWrapper.RuntimeState.Running {
		t.Fatalf("runtime wrapper = %#v", runtimeWrapper)
	}
	if runtimeWrapper.RuntimeState.StartedAt != "2026-06-04T01:00:00Z" {
		t.Fatalf("window runtime startedAt = %q", runtimeWrapper.RuntimeState.StartedAt)
	}

	next := decodeSessionWindowEvent(t, readSSEEventUntil(t, reader, time.Second, "sessions_window").Data)
	if got := strings.Join(next.ThreadOrder, ","); got != "thread-b,thread-a,thread-c" {
		t.Fatalf("reordered window = %s", got)
	}
}

func TestSessionWindowStreamLogsWindowInitializationAndChange(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	threadA := "thread-a"
	threadB := "thread-b"
	threadC := "thread-c"
	rollouts := map[string]string{
		threadA: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-a"}}`,
		}),
		threadB: writeServerRollout(t, []string{
			rolloutEventLine(t, "agent_message", "B"),
		}),
		threadC: writeServerRollout(t, []string{
			`{"timestamp":"2026-06-04T01:01:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-c"}}`,
		}),
	}
	source := newMutableSessionsWithRollouts(
		sessions.Snapshot{
			Sessions: []sessions.SessionSnapshot{
				testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)),
				testSessionSnapshot(threadB, "Beta", "gpt-5.5-mini", "medium", 2, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
			},
		},
		rollouts,
	)
	app := windowStreamTestApp(t, token, source)
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	var logBuffer bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logBuffer)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	response, reader := openSessionWindowStream(t, server.URL, token, "")
	defer response.Body.Close()
	_ = readSSEEventUntil(t, reader, time.Second, "sessions_window")

	source.set(sessions.Snapshot{
		Sessions: []sessions.SessionSnapshot{
			testSessionSnapshot(threadA, "Alpha", "gpt-5.5", "high", 1, 10, time.Date(2026, 6, 4, 1, 0, 0, 0, time.UTC)),
			testSessionSnapshot(threadC, "Gamma", "gpt-5.5", "high", 3, 6, time.Date(2026, 6, 4, 1, 1, 0, 0, time.UTC)),
			testSessionSnapshot(threadB, "Beta", "gpt-5.5-mini", "medium", 2, 18, time.Date(2026, 6, 4, 0, 40, 0, 0, time.UTC)),
		},
	})
	_ = readSSEEventUntil(t, reader, time.Second, "sessions_window")

	logText := logBuffer.String()
	if !strings.Contains(logText, `"action":"window_initialized"`) {
		t.Fatalf("missing window_initialized log: %s", logText)
	}
	if !strings.Contains(logText, `"action":"window_changed"`) {
		t.Fatalf("missing window_changed log: %s", logText)
	}
}

type fakeQuota struct{}

type fakeStaleQuotaWithErrors struct{}

func writeAPKFixture(t *testing.T, dir, name, body string, modTime time.Time) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write apk fixture %s: %v", name, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("set apk fixture time %s: %v", name, err)
	}
}

func newDevUpdateTestApp(t *testing.T, distDir, token string) *App {
	t.Helper()
	return newDevUpdateTestAppWithAllowlist(t, distDir, token, []string{pairing.HashToken(token)})
}

func newDevUpdateTestAppWithAllowlist(t *testing.T, distDir, token string, allowlist []string) *App {
	t.Helper()
	allowlistPath := filepath.Join(t.TempDir(), "dev-allowlist.txt")
	if err := os.WriteFile(allowlistPath, []byte(strings.Join(allowlist, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("write dev allowlist: %v", err)
	}
	cfg := config.Config{
		ApkDistDir:         distDir,
		TokenHash:          pairing.HashToken(token),
		DevUpdateAllowlist: allowlistPath,
	}
	cfg.ApplyDefaults()
	return New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
}

func validPNGFixture() []byte {
	return append(append([]byte{}, pngSignature...), []byte("png-body")...)
}

func attachTestPricing(t *testing.T, app *App) {
	t.Helper()
	t.Setenv("OPENWATCHER_CACHE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<astro-island component-export="TextTokenPricingTables" props="{&quot;tier&quot;:[0,&quot;standard&quot;],&quot;rows&quot;:[1,[[1,[[0,&quot;gpt-5.4&quot;],[0,2.5],[0,0.25],[0,15]]]]]}"></astro-island>`))
	}))
	t.Cleanup(server.Close)
	app.pricing.client = server.Client()
	app.pricing.sourceURL = server.URL
}

func (fakeQuota) Snapshot() (*quota.Snapshot, []string) {
	return &quota.Snapshot{
		Source:   "oauth-api",
		Fresh:    true,
		Status:   quota.StatusOK,
		PlanType: "pro",
		FiveHour: &quota.Window{UsedPercent: 12, RemainingPercent: 88, ResetAt: 1780437338},
		Weekly:   &quota.Window{UsedPercent: 20, RemainingPercent: 80, ResetAt: 1780845860},
	}, nil
}

func (fakeStaleQuotaWithErrors) Snapshot() (*quota.Snapshot, []string) {
	return &quota.Snapshot{
		Source:   "oauth-api",
		Fresh:    false,
		Status:   quota.StatusStale,
		PlanType: "pro",
		FiveHour: &quota.Window{UsedPercent: 12, RemainingPercent: 88, ResetAt: 1780437338},
		Weekly:   &quota.Window{UsedPercent: 20, RemainingPercent: 80, ResetAt: 1780845860},
	}, []string{"quota refresh failed"}
}

type fakeSessions struct {
	rollouts map[string]string
	snapshot *sessions.Snapshot
}

func (f fakeSessions) Snapshot() (sessions.Snapshot, error) {
	if f.snapshot != nil {
		return cloneSessionsSnapshot(*f.snapshot), nil
	}
	peakHour := time.Date(2026, 6, 3, 9, 0, 0, 0, time.UTC)
	return sessions.Snapshot{
		Heatmap24h: sessions.Heatmap24hSnapshot{
			Timezone:      "UTC",
			GeneratedAt:   time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
			PeakHourStart: &peakHour,
			Buckets: []sessions.HeatmapBucket{{
				HourStart:             peakHour,
				InputTokens:           120,
				CachedInputTokens:     20,
				OutputTokens:          30,
				ReasoningOutputTokens: 10,
				TotalTokens:           150,
				ActiveThreads:         1,
			}},
		},
		DailyUsage: sessions.DailyTokenUsage{
			GeneratedAt:           time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
			InputTokens:           120,
			CachedInputTokens:     20,
			OutputTokens:          30,
			ReasoningOutputTokens: 10,
			TotalTokens:           150,
			ActiveSessions:        1,
			ModelTokenBreakdowns: []sessions.DailyModelTokenUsage{{
				Model:                 "gpt-5.4",
				InputTokens:           120,
				CachedInputTokens:     20,
				OutputTokens:          30,
				ReasoningOutputTokens: 10,
				TotalTokens:           150,
			}},
		},
		Sessions: []sessions.SessionSnapshot{{
			ThreadID:                       "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2",
			Title:                          "session title",
			UpdatedAt:                      time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
			Model:                          "gpt-5.4",
			ReasoningEffort:                "high",
			TokensUsedTotal:                5110579,
			ContextUsedTokens:              187224,
			ContextWindow:                  920000,
			ContextPressurePercent:         19,
			ContextCompactThresholdTokens:  850000,
			ContextCompactThresholdPercent: 92,
			LastActiveAgoMinutes:           12,
		}},
	}, nil
}

func (f fakeSessions) SnapshotWithOptions(options sessions.SnapshotOptions) (sessions.Snapshot, error) {
	snapshot, err := f.Snapshot()
	if err != nil {
		return sessions.Snapshot{}, err
	}
	if options.IncludeDailyTrend30d {
		snapshot.DailyTrend30d = fakeDailyTrend30d()
	}
	return snapshot, nil
}

func fakeDailyTrend30d() *sessions.DailyTrend30dSnapshot {
	start := time.Date(2026, 5, 6, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	days := make([]sessions.DailyTrendDay, 0, 30)
	var total int64
	var peak int64
	for index := 0; index < 30; index++ {
		tokens := int64((index + 1) * 1000)
		total += tokens
		if tokens > peak {
			peak = tokens
		}
		days = append(days, sessions.DailyTrendDay{
			Date:        start.AddDate(0, 0, index).Format("2006-01-02"),
			TotalTokens: tokens,
		})
	}
	return &sessions.DailyTrend30dSnapshot{
		Timezone:      "Asia/Shanghai",
		GeneratedAt:   time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC),
		StartDate:     "2026-05-06",
		EndDate:       "2026-06-04",
		TotalTokens:   total,
		AverageTokens: total / 30,
		PeakTokens:    peak,
		Days:          days,
		ModelTokenBreakdowns: []sessions.DailyModelTokenUsage{{
			Model:                 "gpt-5.4",
			InputTokens:           total / 2,
			CachedInputTokens:     total / 5,
			OutputTokens:          total / 4,
			ReasoningOutputTokens: total / 20,
			TotalTokens:           total,
		}},
	}
}

func (s fakeSessions) mustSnapshot(t *testing.T) sessions.Snapshot {
	t.Helper()
	snapshot, err := s.Snapshot()
	if err != nil {
		t.Fatalf("fake sessions snapshot: %v", err)
	}
	return snapshot
}

type mutableSessions struct {
	mu       sync.RWMutex
	current  sessions.Snapshot
	rollouts map[string]string
}

func newMutableSessions(snapshot sessions.Snapshot) *mutableSessions {
	return &mutableSessions{current: cloneSessionsSnapshot(snapshot)}
}

func newMutableSessionsWithRollouts(snapshot sessions.Snapshot, rollouts map[string]string) *mutableSessions {
	copiedRollouts := make(map[string]string, len(rollouts))
	for threadID, rolloutPath := range rollouts {
		copiedRollouts[threadID] = rolloutPath
	}
	return &mutableSessions{
		current:  cloneSessionsSnapshot(snapshot),
		rollouts: copiedRollouts,
	}
}

func (s *mutableSessions) Snapshot() (sessions.Snapshot, error) {
	return s.snapshot(), nil
}

func (s *mutableSessions) SnapshotWithOptions(options sessions.SnapshotOptions) (sessions.Snapshot, error) {
	snapshot := s.snapshot()
	if options.IncludeDailyTrend30d {
		snapshot.DailyTrend30d = fakeDailyTrend30d()
	}
	return snapshot, nil
}

func (s *mutableSessions) snapshot() sessions.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSessionsSnapshot(s.current)
}

func (s *mutableSessions) set(snapshot sessions.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = cloneSessionsSnapshot(snapshot)
}

func (s *mutableSessions) RolloutPathForThread(threadID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if path, ok := s.rollouts[threadID]; ok {
		return path, nil
	}
	return "", sessions.ErrThreadNotFound
}

func cloneSessionsSnapshot(input sessions.Snapshot) sessions.Snapshot {
	output := sessions.Snapshot{
		Heatmap24h:    input.Heatmap24h,
		DailyUsage:    input.DailyUsage,
		DailyTrend30d: input.DailyTrend30d,
		Sessions:      append([]sessions.SessionSnapshot(nil), input.Sessions...),
	}
	output.Heatmap24h.Buckets = append([]sessions.HeatmapBucket(nil), input.Heatmap24h.Buckets...)
	if input.DailyTrend30d != nil {
		copiedTrend := *input.DailyTrend30d
		copiedTrend.Days = append([]sessions.DailyTrendDay(nil), input.DailyTrend30d.Days...)
		output.DailyTrend30d = &copiedTrend
	}
	return output
}

func (f fakeSessions) RolloutPathForThread(threadID string) (string, error) {
	if path, ok := f.rollouts[threadID]; ok {
		return path, nil
	}
	return "", sessions.ErrThreadNotFound
}

type sseTestEvent struct {
	Name string
	ID   string
	Data string
}

func streamTestApp(t *testing.T, token, threadID, rolloutPath string) *App {
	t.Helper()
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(
		filepath.Join(t.TempDir(), "config.json"),
		cfg,
		false,
		fakeQuota{},
		fakeSessions{rollouts: map[string]string{threadID: rolloutPath}},
	)
	app.clock = func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) }
	return app
}

func windowStreamTestApp(t *testing.T, token string, source SessionSource) *App {
	t.Helper()
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(
		filepath.Join(t.TempDir(), "config.json"),
		cfg,
		false,
		fakeQuota{},
		source,
	)
	app.clock = func() time.Time { return time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC) }
	app.streamTailInterval = 10 * time.Millisecond
	app.streamHeartbeatInterval = 40 * time.Millisecond
	return app
}

func openStream(t *testing.T, baseURL, token, threadID string) (*http.Response, *bufio.Reader) {
	return openStreamWithQuery(t, baseURL, token, threadID, "")
}

func openStreamWithQuery(t *testing.T, baseURL, token, threadID, query string) (*http.Response, *bufio.Reader) {
	t.Helper()
	url := baseURL + "/api/sessions/" + threadID + "/stream"
	if query != "" {
		url += "?" + query
	}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new stream request: %v", err)
	}
	request.Header.Set("X-OpenWatcher-Token", token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		t.Fatalf("stream status = %d", response.StatusCode)
	}
	return response, bufio.NewReader(response.Body)
}

func openSessionWindowStream(t *testing.T, baseURL, token, query string) (*http.Response, *bufio.Reader) {
	t.Helper()
	url := baseURL + "/api/sessions/stream"
	if query != "" {
		url += "?" + query
	}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new session window request: %v", err)
	}
	request.Header.Set("X-OpenWatcher-Token", token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("open session window stream: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		t.Fatalf("session window stream status=%d body=%s", response.StatusCode, string(body))
	}
	return response, bufio.NewReader(response.Body)
}

func openStatusStream(t *testing.T, baseURL, token string) (*http.Response, *bufio.Reader) {
	t.Helper()
	return openStatusStreamWithQuery(t, baseURL, token, "")
}

func openStatusStreamWithQuery(t *testing.T, baseURL, token string, query string) (*http.Response, *bufio.Reader) {
	t.Helper()
	url := baseURL + "/api/status/stream"
	if query != "" {
		url += "?" + query
	}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new status stream request: %v", err)
	}
	request.Header.Set("X-OpenWatcher-Token", token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("open status stream: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		t.Fatalf("status stream status = %d", response.StatusCode)
	}
	return response, bufio.NewReader(response.Body)
}

func readSSEEventUntil(t *testing.T, reader *bufio.Reader, timeout time.Duration, eventName string) sseTestEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE event %q", eventName)
		default:
		}
		event := readSSEEvent(t, reader, timeout)
		if event.Name == eventName {
			return event
		}
	}
}

func readSSEEvent(t *testing.T, reader *bufio.Reader, timeout time.Duration) sseTestEvent {
	t.Helper()
	type result struct {
		event sseTestEvent
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		event, err := readSSEEventBlocking(reader)
		resultCh <- result{event: event, err: err}
	}()
	select {
	case result := <-resultCh:
		if result.err != nil {
			t.Fatalf("read SSE event: %v", result.err)
		}
		return result.event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for SSE event")
		return sseTestEvent{}
	}
}

func readSSEEventBlocking(reader *bufio.Reader) (sseTestEvent, error) {
	var event sseTestEvent
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return sseTestEvent{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return event, nil
		}
		switch {
		case strings.HasPrefix(line, "event: "):
			event.Name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "id: "):
			event.ID = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			event.Data = strings.TrimPrefix(line, "data: ")
		}
	}
}

func decodeAgentMessage(t *testing.T, data string) sessions.AgentMessage {
	t.Helper()
	var message sessions.AgentMessage
	if err := json.Unmarshal([]byte(data), &message); err != nil {
		t.Fatalf("decode agent message: %v data=%s", err, data)
	}
	return message
}

func decodeRuntimeState(t *testing.T, data string) sessions.RuntimeState {
	t.Helper()
	var state sessions.RuntimeState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		t.Fatalf("decode runtime state: %v data=%s", err, data)
	}
	return state
}

func decodeSessionWindowEvent(t *testing.T, data string) sessionWindowEvent {
	t.Helper()
	var event sessionWindowEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("decode session window event: %v data=%s", err, data)
	}
	return event
}

func decodeSessionWindowRuntimeStateEvent(t *testing.T, data string) sessionWindowRuntimeStateEvent {
	t.Helper()
	var event sessionWindowRuntimeStateEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("decode session window runtime event: %v data=%s", err, data)
	}
	return event
}

func testSessionSnapshot(
	threadID string,
	title string,
	model string,
	reasoning string,
	tokensUsedTotal int64,
	lastActiveAgoMinutes int64,
	updatedAt time.Time,
) sessions.SessionSnapshot {
	return sessions.SessionSnapshot{
		ThreadID:                       threadID,
		Title:                          title,
		UpdatedAt:                      updatedAt,
		Model:                          model,
		ReasoningEffort:                reasoning,
		TokensUsedTotal:                tokensUsedTotal,
		ContextUsedTokens:              180_000,
		ContextWindow:                  920_000,
		ContextPressurePercent:         20,
		ContextCompactThresholdTokens:  850_000,
		ContextCompactThresholdPercent: 92,
		LastActiveAgoMinutes:           lastActiveAgoMinutes,
	}
}

func writeServerRollout(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	appendServerRollout(t, path, lines)
	return path
}

func appendServerRollout(t *testing.T, path string, lines []string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open rollout: %v", err)
	}
	defer file.Close()
	for _, line := range lines {
		if _, err := file.WriteString(line + "\n"); err != nil {
			t.Fatalf("append rollout: %v", err)
		}
	}
}

var rolloutLineCounter int64

func rolloutEventLine(t *testing.T, payloadType, message string) string {
	t.Helper()
	counter := atomic.AddInt64(&rolloutLineCounter, 1)
	payload := map[string]any{
		"timestamp": time.Date(2026, 6, 4, 1, 0, int(counter%60), 0, time.UTC).Format(time.RFC3339Nano),
		"type":      "event_msg",
		"payload": map[string]any{
			"type":    payloadType,
			"message": message,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal rollout event: %v", err)
	}
	return string(data)
}
