package widgetapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"openwatcher/desktop-app/widget/internal/widgetvm"
	"strings"
	"sync"
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
func TestSnapshotAndStreamQueriesSeparateMonthlyTrend(t *testing.T) {
	u, _ := url.Parse(loopbackEndpoint + "/api/status")
	req, err := request(context.Background(), u, "secret", false, false)
	if err != nil || req.URL.Query().Has("includeDailyTrend30d") || req.URL.Query().Get("includeSessions") != "0" || req.Header.Get("X-OpenWatcher-Token") != "secret" {
		t.Fatalf("%v %#v", err, req)
	}
	streamURL, _ := url.Parse(loopbackEndpoint + "/api/status/stream")
	streamReq, err := request(context.Background(), streamURL, "secret", true, true)
	if err != nil || streamReq.URL.Query().Get("includeDailyTrend30d") != "1" || streamReq.Header.Get("Accept") != "text/event-stream" {
		t.Fatalf("%v %#v", err, streamReq)
	}
}

type fixedClientClock struct{ now time.Time }

func (c fixedClientClock) Now() time.Time                     { return c.now }
func (fixedClientClock) After(time.Duration) <-chan time.Time { return make(chan time.Time) }

type recordingTrendStore struct {
	mu    sync.Mutex
	trend *widgetvm.Trend30d
	saves int
	saved chan struct{}
}

func (s *recordingTrendStore) LoadTrend30d() *widgetvm.Trend30d {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trend
}
func (s *recordingTrendStore) SaveTrend30d(trend *widgetvm.Trend30d) error {
	s.mu.Lock()
	s.trend = trend
	s.saves++
	saved := s.saved
	s.mu.Unlock()
	if saved != nil {
		select {
		case saved <- struct{}{}:
		default:
		}
	}
	return nil
}
func (s *recordingTrendStore) saveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saves
}

func TestCurrentTrendCacheLoadsAndStreamSnapshotPreservesIt(t *testing.T) {
	clock := fixedClientClock{now: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)}
	store := &recordingTrendStore{trend: &widgetvm.Trend30d{EndDate: "2026-07-10", TotalTokens: 42}}
	c := NewClientWithClock(loopbackEndpoint, NoTokenSource{}, clock, store)
	if c.needsDailyTrend30d() || c.state.Trend30d == nil || c.state.Trend30d.TotalTokens != 42 {
		t.Fatalf("cached trend was not restored: %+v", c.state)
	}
	c.applyEvent("status_snapshot", `{"observedAt":"2026-07-11T12:01:00+08:00","heatmap24h":{"timezone":"Asia/Shanghai"}}`)
	if c.state.Trend30d == nil || c.state.Trend30d.TotalTokens != 42 {
		t.Fatalf("stream snapshot dropped cached trend: %+v", c.state)
	}
}

func TestFreshStreamTrendIsSavedAndStaleCacheRequestsReplacement(t *testing.T) {
	clock := fixedClientClock{now: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)}
	store := &recordingTrendStore{trend: &widgetvm.Trend30d{EndDate: "2026-07-09"}}
	c := NewClientWithClock(loopbackEndpoint, NoTokenSource{}, clock, store)
	if !c.needsDailyTrend30d() || c.state.Trend30d != nil {
		t.Fatalf("stale cache was accepted: %+v", c.state)
	}
	c.applyEvent("status_snapshot", `{"observedAt":"2026-07-11T12:01:00+08:00","dailyTrend30d":{"endDate":"2026-07-10","totalTokens":99}}`)
	if store.saveCount() != 1 || c.state.Trend30d == nil || c.state.Trend30d.TotalTokens != 99 || c.needsDailyTrend30d() {
		t.Fatalf("fresh trend was not persisted: saves=%d state=%+v", store.saveCount(), c.state)
	}
}

func TestRunEmitsBasicSnapshotBeforeUncachedMonthlyTrend(t *testing.T) {
	streamStarted := make(chan struct{})
	releaseTrend := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			if r.URL.Query().Has("includeDailyTrend30d") {
				t.Errorf("basic snapshot unexpectedly requested monthly trend: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"observedAt":"2026-07-11T12:00:00+08:00","heatmap24h":{"timezone":"Asia/Shanghai","buckets":[]},"dailyUsage":{"totalTokens":7}}`))
		case "/api/status/stream":
			if r.URL.Query().Get("includeDailyTrend30d") != "1" {
				t.Errorf("uncached stream did not request monthly trend: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			close(streamStarted)
			select {
			case <-releaseTrend:
				_, _ = w.Write([]byte("event: status_snapshot\ndata: {\"observedAt\":\"2026-07-11T12:00:01+08:00\",\"dailyTrend30d\":{\"endDate\":\"2026-07-10\",\"totalTokens\":99}}\n\n"))
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clock := fixedClientClock{now: time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)}
	store := &recordingTrendStore{saved: make(chan struct{}, 1)}
	client := NewClientWithClock(server.URL, fixedToken{}, clock, store)
	states := make(chan widgetvm.State, 8)
	go client.Run(ctx, func(state widgetvm.State) { states <- state })

	select {
	case state := <-states:
		if state.Status != widgetvm.Online || state.Today == nil || state.Today.TotalTokens != 7 || state.Trend30d != nil {
			t.Fatalf("basic state was not emitted first: %+v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("basic state was blocked by monthly trend")
	}
	select {
	case <-streamStarted:
	case <-time.After(time.Second):
		t.Fatal("monthly trend stream did not start")
	}
	close(releaseTrend)
	select {
	case state := <-states:
		if state.Trend30d == nil || state.Trend30d.TotalTokens != 99 {
			t.Fatalf("monthly trend was not applied asynchronously: state=%+v", state)
		}
	case <-time.After(time.Second):
		t.Fatal("monthly trend was not emitted")
	}
	select {
	case <-store.saved:
	case <-time.After(time.Second):
		t.Fatal("monthly trend was not persisted")
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
