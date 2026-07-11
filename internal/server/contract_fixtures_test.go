package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/internal/buildinfo"
	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
	"openwatcher/testsupport/contracts"
)

const contractToken = "contract-token-0123456789abcdef0123456789"

func TestContractFixturesForWatchProtocol(t *testing.T) {
	payloads := serverContractFixtures(t)
	for name, data := range payloads {
		contracts.AssertFixture(t, name, data)
	}
}

func serverContractFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	app, codexHome := newContractApp(t)

	var health healthResponse
	healthBody := requestContractJSON(t, app, "/healthz", "", http.StatusOK, &health)

	var status statusResponse
	statusBody := requestContractJSON(t, app, "/api/status?includeDailyTrend30d=1", contractToken, http.StatusOK, &status)

	var unauthorized map[string]any
	unauthorizedBody := requestContractJSON(t, app, "/api/status", "", http.StatusUnauthorized, &unauthorized)

	payloads := map[string][]byte{
		"healthz.ok.json":          healthBody,
		"status.ok.json":           statusBody,
		"status.unauthorized.json": unauthorizedBody,
		"status.sse":               buildContractStatusSSE(t, status),
	}
	for name, data := range payloads {
		contracts.RejectPrivateFragments(t, name, data, contractToken, pairing.HashToken(contractToken), codexHome, os.Getenv("HOME"))
	}
	return payloads
}

func newContractApp(t *testing.T) (*App, string) {
	t.Helper()
	t.Setenv("OPENWATCHER_CACHE_DIR", t.TempDir())

	oldCommit, oldBuiltAt := buildinfo.Commit, buildinfo.BuiltAt
	buildinfo.Commit = "contract"
	buildinfo.BuiltAt = "2026-06-10T15:00:00Z"
	t.Cleanup(func() {
		buildinfo.Commit = oldCommit
		buildinfo.BuiltAt = oldBuiltAt
	})

	codexHome := filepath.Join(t.TempDir(), "codex")
	if err := os.MkdirAll(filepath.Join(codexHome, "sessions"), 0o700); err != nil {
		t.Fatalf("mkdir codex sessions fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"fixture":true}`), 0o600); err != nil {
		t.Fatalf("write codex auth fixture: %v", err)
	}

	cfg := config.Config{
		Listen:        "127.0.0.1:8787",
		PublicBaseURL: "http://192.168.1.12:8787",
		TokenHash:     pairing.HashToken(contractToken),
		CodexHome:     codexHome,
	}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.clock = func() time.Time { return time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC) }
	app.pricing.client = &http.Client{Transport: failingPricingTransport{}}
	return app, codexHome
}

type failingPricingTransport struct{}

func (failingPricingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("pricing fetch disabled for contract fixture")
}

func requestContractJSON(t *testing.T, app *App, path string, token string, wantStatus int, target any) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-OpenWatcher-Token", token)
	}
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != wantStatus {
		t.Fatalf("%s status = %d, want %d body=%s", path, res.Code, wantStatus, res.Body.String())
	}
	if err := json.Unmarshal(res.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %s response: %v body=%s", path, err, res.Body.String())
	}
	return marshalContractJSON(t, target)
}

func marshalContractJSON(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal contract JSON: %v", err)
	}
	return append(data, '\n')
}

func buildContractStatusSSE(t *testing.T, response statusResponse) []byte {
	t.Helper()
	var buf bytes.Buffer
	writeContractSSE(t, &buf, "status_snapshot", statusEventID("status_snapshot", response.ObservedAt), statusSnapshotEvent{
		Type:           "status_snapshot",
		statusResponse: response,
	})
	writeContractSSE(t, &buf, "status_quota", statusEventID("status_quota", response.ObservedAt), statusQuotaEvent{
		Type:       "status_quota",
		ObservedAt: response.ObservedAt,
		Quota:      response.Quota,
	})
	writeContractSSE(t, &buf, "status_heatmap24h", statusEventID("status_heatmap24h", response.ObservedAt), statusHeatmapEvent{
		Type:       "status_heatmap24h",
		ObservedAt: response.ObservedAt,
		Heatmap24h: response.Heatmap24h,
		Heatmap7d:  response.Heatmap7d,
		DailyUsage: response.DailyUsage,
	})
	writeContractSSE(t, &buf, "status_sessions", statusEventID("status_sessions", response.ObservedAt), statusSessionsEvent{
		Type:       "status_sessions",
		ObservedAt: response.ObservedAt,
		Sessions:   statusResponseSessions(response),
	})
	writeContractSSE(t, &buf, "status_errors", statusEventID("status_errors", response.ObservedAt), statusErrorsEvent{
		Type:       "status_errors",
		ObservedAt: response.ObservedAt,
		Errors:     response.Errors,
	})
	writeContractSSE(t, &buf, "heartbeat", statusEventID("status_heartbeat", response.ObservedAt), statusHeartbeatEvent{
		Type:      "heartbeat",
		CreatedAt: response.ObservedAt,
	})
	return buf.Bytes()
}

func writeContractSSE(t *testing.T, buf *bytes.Buffer, eventName, eventID string, payload any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s SSE payload: %v", eventName, err)
	}
	buf.WriteString("event: " + eventName + "\n")
	buf.WriteString("id: " + eventID + "\n")
	for _, line := range strings.Split(string(data), "\n") {
		buf.WriteString("data: " + line + "\n")
	}
	buf.WriteString("\n")
}
