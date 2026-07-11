package widgetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"openwatcher/desktop-app/widget/internal/widgetvm"
	"strings"
	"testing"
	"time"
)

const loopbackEndpoint = "http://127.0.0.1:9876"

type fixedToken struct{}

func (fixedToken) Token() (string, error) { return "test-token", nil }
func TestValidatedURLOnlyAllowsPlainLoopbackRoot(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:8787", "http://[::1]:8787"} {
		if _, err := validatedURL(raw, "/api/status"); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{"https://127.0.0.1", "http://example.test", "http://user@127.0.0.1", "http://127.0.0.1/#x", "http://127.0.0.1/api"} {
		if _, err := validatedURL(raw, "/api/status"); err == nil {
			t.Fatal("accepted", raw)
		}
	}
}
func TestRequiredQueryAndHeaderAreSetInGo(t *testing.T) {
	u, _ := url.Parse(loopbackEndpoint + "/api/status")
	req, err := request(context.Background(), u, "secret", false)
	if err != nil || req.URL.Query().Get("includeDailyTrend30d") != "1" || req.URL.Query().Get("includeSessions") != "0" || req.Header.Get("X-OpenWatcher-Token") != "secret" {
		t.Fatalf("%v %#v", err, req)
	}
}
func TestSnapshotViewModelDoesNotExposeConversationData(t *testing.T) {
	var dto response
	if err := json.Unmarshal([]byte(`{"observedAt":"2026-07-11T10:00:00Z","quota":{"fresh":true,"status":"ok"},"sessions":[{"title":"private"}]}`), &dto); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(dto.state())
	if strings.Contains(string(b), "sessions") || strings.Contains(string(b), "title") || strings.Contains(string(b), "messages") {
		t.Fatalf("private fields leaked: %s", b)
	}
}
func TestSSEPartialHeartbeatAndUnknown(t *testing.T) {
	c := NewClient(loopbackEndpoint, NoTokenSource{})
	c.applyEvent("status_quota", `{"quota":{"fresh":true,"status":"ok","fiveHour":{"remainingPercent":81}}}`)
	c.applyEvent("status_heatmap24h", `{"heatmap24h":{"timezone":"Asia/Shanghai","buckets":[{"hourStart":"2026-07-11T10:00:00+08:00","totalTokens":4}]},"dailyUsage":{"totalTokens":4}}`)
	c.applyEvent("heartbeat", "")
	c.applyEvent("unknown", `{"ignored":true}`)
	c.mu.Lock()
	s := c.state
	c.mu.Unlock()
	if s.Quota == nil || s.Heatmap24h == nil || s.Today == nil {
		t.Fatalf("%+v", s)
	}
}
func TestSSESupportsMultilineDataAndCrossDayRequestsFullRefresh(t *testing.T) {
	ch := make(chan sseEvent, 2)
	go func() {
		_ = scanSSE(context.Background(), bytes.NewBufferString("event: status_snapshot\ndata: {\"observedAt\":\ndata: \"2026-07-12T00:01:00+08:00\"}\n\n"), ch)
	}()
	event := <-ch
	if !strings.Contains(event.data, "\n") {
		t.Fatalf("multiline data lost: %q", event.data)
	}
	c := NewClient(loopbackEndpoint, NoTokenSource{})
	c.state.ObservedAt = "2026-07-11T23:59:00+08:00"
	c.applyEvent("status_snapshot", `{"observedAt":"2026-07-12T00:01:00+08:00"}`)
	if len(c.refresh) != 1 {
		t.Fatal("cross-day update did not schedule full GET")
	}
}
func TestHeartbeatAcrossAPITimezoneDayRequestsFullRefresh(t *testing.T) {
	c := NewClient(loopbackEndpoint, NoTokenSource{})
	c.state.ObservedAt = "2026-07-11T23:59:00+08:00"
	c.applyEvent("heartbeat", `{"type":"heartbeat","createdAt":"2026-07-12T00:00:01+08:00"}`)
	if len(c.refresh) != 1 {
		t.Fatal("cross-day heartbeat did not schedule full GET")
	}
}
func TestManualRefreshCoalescesToOneLoopSignal(t *testing.T) {
	c := NewClient(loopbackEndpoint, NoTokenSource{})
	c.Refresh()
	c.Refresh()
	c.Refresh()
	if len(c.refresh) != 1 {
		t.Fatal(len(c.refresh))
	}
}
func TestUnauthorizedIsInvalidCredential(t *testing.T) {
	if classifyHTTPStatus(http.StatusUnauthorized) != widgetvm.Invalid || classifyHTTPStatus(http.StatusBadGateway) != widgetvm.Offline {
		t.Fatal("wrong HTTP classification")
	}
}
func TestFreshnessThresholds(t *testing.T) {
	c := NewClient(loopbackEndpoint, NoTokenSource{})
	c.state.Status = widgetvm.Online
	c.lastActivity = time.Now().Add(-31 * time.Second)
	c.checkStale()
	if c.state.Status != widgetvm.Reconnecting {
		t.Fatal(c.state.Status)
	}
	c.lastActivity = time.Now().Add(-61 * time.Second)
	c.checkStale()
	if c.state.Status != widgetvm.Stale {
		t.Fatal(c.state.Status)
	}
}
func TestConnectionFailureRetainsStaleDataAfterSixtySeconds(t *testing.T) {
	c := NewClient(loopbackEndpoint, fixedToken{})
	c.state.Quota = &widgetvm.Quota{Fresh: true}
	c.lastActivity = time.Now().Add(-61 * time.Second)
	c.connectionUnavailable()
	if c.state.Status != widgetvm.Stale || c.state.Quota == nil {
		t.Fatalf("%+v", c.state)
	}
}
func TestSSEStreamHasPerEventRatherThanCumulativeLimit(t *testing.T) {
	var input strings.Builder
	const events = 2600
	payload := strings.Repeat("x", 900)
	for i := 0; i < events; i++ {
		input.WriteString("event: heartbeat\ndata: ")
		input.WriteString(payload)
		input.WriteString("\n\n")
	}
	out := make(chan sseEvent, 16)
	done := make(chan error, 1)
	go func() { done <- scanSSE(context.Background(), strings.NewReader(input.String()), out) }()
	count := 0
	for range out {
		count++
	}
	if err := <-done; err != nil || count != events {
		t.Fatalf("events=%d err=%v", count, err)
	}
}
func TestLoopbackTransportNeverUsesProxyAndBoundsHeaders(t *testing.T) {
	c := NewClient(loopbackEndpoint, fixedToken{})
	transport, ok := c.streamHTTP.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil || transport.ResponseHeaderTimeout != 12*time.Second || transport.MaxResponseHeaderBytes != 64<<10 {
		t.Fatalf("%#v", c.streamHTTP.Transport)
	}
}
func TestRetryJitterNeverExceedsDocumentedBackoffCap(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := retryDelay(30 * time.Second)
		if got < 25500*time.Millisecond || got > 30*time.Second {
			t.Fatal(got)
		}
	}
}
