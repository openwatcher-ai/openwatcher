package widgetapi

import (
	"encoding/json"
	"net/url"
	"openwatcher/desktop-app/widget/internal/widgetvm"
	"strings"
	"testing"
	"time"
)

func TestValidatedURL(t *testing.T) {
	for _, base := range []string{"http://127.0.0.1:8787", "https://example.test/"} {
		u, e := validatedURL(base, "/api/status")
		if e != nil || u.Path != "/api/status" {
			t.Fatalf("%s: %v %v", base, u, e)
		}
	}
	if _, e := validatedURL("file:///tmp/x", "/"); e == nil {
		t.Fatal("file URL accepted")
	}
}
func TestRequiredQuery(t *testing.T) {
	u, _ := url.Parse(DefaultEndpoint + "/api/status")
	q := u.Query()
	q.Set("includeDailyTrend30d", "1")
	q.Set("includeSessions", "0")
	if q.Get("includeSessions") != "0" {
		t.Fatal("missing query")
	}
}

func TestSnapshotViewModelDoesNotExposeConversationData(t *testing.T) {
	var dto response
	if err := json.Unmarshal([]byte(`{"observedAt":"2026-07-11T10:00:00Z","quota":{"fresh":true,"status":"ok"},"dailyUsage":{"totalTokens":12},"sessions":[{"title":"private"}]}`), &dto); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(dto.state())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "sessions") || strings.Contains(string(b), "title") {
		t.Fatalf("private fields leaked: %s", b)
	}
}

func TestSSEEventsUpdateOnlyAllowedState(t *testing.T) {
	c := NewClient(DefaultEndpoint, NoTokenSource{})
	c.applyEvent("status_quota", `{"quota":{"fresh":true,"status":"ok","fiveHour":{"remainingPercent":81}}}`)
	c.applyEvent("status_heatmap24h", `{"heatmap24h":{"timezone":"Asia/Shanghai","buckets":[{"hourStart":"2026-07-11T10:00:00+08:00","totalTokens":4}]},"dailyUsage":{"totalTokens":4}}`)
	c.mu.Lock()
	state := c.state
	c.mu.Unlock()
	if state.Quota == nil || state.Quota.FiveHour == nil || state.Heatmap24h == nil || state.Today == nil {
		t.Fatalf("partial events not applied: %+v", state)
	}
}

func TestConnectionStateThresholds(t *testing.T) {
	c := NewClient(DefaultEndpoint, NoTokenSource{})
	c.state.Status = widgetvm.Online
	c.lastEvent = time.Now().Add(-31 * time.Second)
	c.checkStale()
	if c.state.Status != widgetvm.Reconnecting {
		t.Fatalf("want reconnecting, got %s", c.state.Status)
	}
	c.lastEvent = time.Now().Add(-61 * time.Second)
	c.checkStale()
	if c.state.Status != widgetvm.Stale {
		t.Fatalf("want stale, got %s", c.state.Status)
	}
}
