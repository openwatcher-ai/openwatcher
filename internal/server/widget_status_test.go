package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
	"openwatcher/internal/sessions"
)

func TestStatusOmitsSessionsWhenRequestedWithoutChangingDefault(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})

	request := func(query string) string {
		req := httptest.NewRequest(http.MethodGet, "/api/status"+query, nil)
		req.Header.Set("X-OpenWatcher-Token", token)
		res := httptest.NewRecorder()
		app.Handler().ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
		}
		return res.Body.String()
	}

	if body := request(""); !strings.Contains(body, `"sessions":[`) {
		t.Fatalf("default status did not preserve sessions array: %s", body)
	}
	if body := request("?includeSessions=0"); strings.Contains(body, `"sessions"`) {
		t.Fatalf("sessions were not omitted: %s", body)
	}
}

func TestStatusKeepsEmptySessionsArrayByDefault(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	emptySnapshot := sessions.Snapshot{}
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{snapshot: &emptySnapshot})
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	app.Handler().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", res.Code, res.Body.String())
	}
	if body := res.Body.String(); !strings.Contains(body, `"sessions":[]`) {
		t.Fatalf("default empty sessions were not preserved as an array: %s", body)
	}
}

func TestStatusStreamOmitsSessionsForInitialSnapshot(t *testing.T) {
	token := "0123456789abcdef0123456789abcdef"
	cfg := config.Config{TokenHash: pairing.HashToken(token)}
	cfg.ApplyDefaults()
	source := newMutableSessions(fakeSessions{}.mustSnapshot(t))
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, source)
	app.statusStreamPollInterval = 5 * time.Millisecond
	app.streamHeartbeatInterval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/status/stream?includeSessions=0&includeDailyTrend30d=1", nil).WithContext(ctx)
	req.Header.Set("X-OpenWatcher-Token", token)
	res := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		app.Handler().ServeHTTP(res, req)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	next := source.snapshot()
	next.Sessions[0].Title = "changed title"
	source.set(next)
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("status stream did not stop after cancellation")
	}
	body := res.Body.String()
	if !strings.Contains(body, "event: status_snapshot") || strings.Contains(body, `"sessions"`) || strings.Contains(body, "event: status_sessions") {
		t.Fatalf("unexpected stream body: %s", body)
	}
	if !strings.Contains(body, `"dailyTrend30d"`) {
		t.Fatalf("daily trend missing from stream body: %s", body)
	}
}

func TestWidgetHandlerIsIndependentLoopbackReadOnlyRoute(t *testing.T) {
	watchToken := "0123456789abcdef0123456789abcdef"
	widgetToken := "abcdef0123456789abcdef0123456789"
	cfg := config.Config{TokenHash: pairing.HashToken(watchToken), WidgetTokenHash: pairing.HashToken(widgetToken)}
	cfg.ApplyDefaults()
	app := New(filepath.Join(t.TempDir(), "config.json"), cfg, false, fakeQuota{}, fakeSessions{})
	app.SetNoAuth(true)

	request := func(method, path, remote, token string) (int, string) {
		req := httptest.NewRequest(method, path, nil)
		req.RemoteAddr = remote
		if token != "" {
			req.Header.Set("X-OpenWatcher-Token", token)
		}
		res := httptest.NewRecorder()
		app.WidgetHandler().ServeHTTP(res, req)
		return res.Code, res.Body.String()
	}

	if code, body := request(http.MethodGet, "/api/status", "127.0.0.1:1234", widgetToken); code != http.StatusOK || strings.Contains(body, `"sessions"`) {
		t.Fatalf("widget status = %d body=%s", code, body)
	}
	if code, _ := request(http.MethodGet, "/api/status", "[::1]:1234", widgetToken); code != http.StatusOK {
		t.Fatalf("widget IPv6 status = %d", code)
	}
	for _, tc := range []struct {
		method, path, remote, token string
		want                        int
	}{
		{http.MethodGet, "/api/status", "127.0.0.1:1234", "", http.StatusUnauthorized},
		{http.MethodGet, "/api/status", "127.0.0.1:1234", watchToken, http.StatusUnauthorized},
		{http.MethodGet, "/api/status", "192.168.1.2:1234", widgetToken, http.StatusForbidden},
		{http.MethodPost, "/api/status", "127.0.0.1:1234", widgetToken, http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/other", "127.0.0.1:1234", widgetToken, http.StatusNotFound},
	} {
		if got, _ := request(tc.method, tc.path, tc.remote, tc.token); got != tc.want {
			t.Fatalf("widget %s %s from %s = %d, want %d", tc.method, tc.path, tc.remote, got, tc.want)
		}
	}
}
