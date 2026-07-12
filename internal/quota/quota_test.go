package quota

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseUsageSupportsResetAtAndResetAfterSeconds(t *testing.T) {
	now := time.Unix(1000, 0)
	payload := []byte(`{
	  "plan_type": "pro",
	  "rate_limit": {
	    "primary_window": {
	      "used_percent": 12.345,
	      "reset_at": 2000
	    },
	    "secondary_window": {
	      "used_percent": "20",
	      "reset_after_seconds": 60
	    }
	  }
	}`)

	snapshot, err := ParseUsage(payload, now)
	if err != nil {
		t.Fatalf("ParseUsage() error = %v", err)
	}
	if snapshot.PlanType != "pro" {
		t.Fatalf("PlanType = %q", snapshot.PlanType)
	}
	if snapshot.FiveHour == nil || snapshot.FiveHour.UsedPercent != 12.35 {
		t.Fatalf("five hour window = %#v", snapshot.FiveHour)
	}
	if snapshot.FiveHour.RemainingPercent != 87.65 || snapshot.FiveHour.ResetAt != 2000 {
		t.Fatalf("five hour derived fields = %#v", snapshot.FiveHour)
	}
	if snapshot.Weekly == nil || snapshot.Weekly.ResetAt != 1060 {
		t.Fatalf("weekly window = %#v", snapshot.Weekly)
	}
}

func TestParseUsageAllowsMissingCreditsAndWindow(t *testing.T) {
	payload := []byte(`{
	  "rate_limit": {
	    "primary_window": {"used_percent": 0, "resets_at": 3000}
	  }
	}`)
	snapshot, err := ParseUsage(payload, time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("ParseUsage() error = %v", err)
	}
	if snapshot.FiveHour == nil || snapshot.FiveHour.ResetAt != 3000 {
		t.Fatalf("five hour window = %#v", snapshot.FiveHour)
	}
	if snapshot.Weekly != nil {
		t.Fatalf("weekly = %#v, want nil", snapshot.Weekly)
	}
}

func TestParseUsageClassifiesSoleWeeklyPrimaryByDuration(t *testing.T) {
	now := time.Unix(1000, 0)
	payload := []byte(`{
	  "plan_type": "pro",
	  "rate_limit": {
	    "primary_window": {
	      "used_percent": 31,
	      "limit_window_seconds": 604800,
	      "reset_after_seconds": 471600
	    }
	  }
	}`)

	snapshot, err := ParseUsage(payload, now)
	if err != nil {
		t.Fatalf("ParseUsage() error = %v", err)
	}
	if snapshot.FiveHour != nil {
		t.Fatalf("five hour = %#v, want nil", snapshot.FiveHour)
	}
	if snapshot.Weekly == nil {
		t.Fatal("weekly = nil")
	}
	if snapshot.Weekly.RemainingPercent != 69 || snapshot.Weekly.ResetAt != 472600 {
		t.Fatalf("weekly = %#v", snapshot.Weekly)
	}
}

func TestParseUsageClassifiesReorderedWindowsByDuration(t *testing.T) {
	payload := []byte(`{
	  "rate_limit": {
	    "primary_window": {
	      "used_percent": 25,
	      "limit_window_seconds": 604800,
	      "reset_at": 7000
	    },
	    "secondary_window": {
	      "used_percent": 10,
	      "limit_window_seconds": 18000,
	      "reset_at": 2000
	    }
	  }
	}`)

	snapshot, err := ParseUsage(payload, time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("ParseUsage() error = %v", err)
	}
	if snapshot.FiveHour == nil || snapshot.FiveHour.RemainingPercent != 90 || snapshot.FiveHour.ResetAt != 2000 {
		t.Fatalf("five hour = %#v", snapshot.FiveHour)
	}
	if snapshot.Weekly == nil || snapshot.Weekly.RemainingPercent != 75 || snapshot.Weekly.ResetAt != 7000 {
		t.Fatalf("weekly = %#v", snapshot.Weekly)
	}
}

func TestParseUsageDoesNotMislabelUnsupportedExplicitDuration(t *testing.T) {
	payload := []byte(`{
	  "rate_limit": {
	    "primary_window": {
	      "used_percent": 50,
	      "limit_window_seconds": 86400,
	      "reset_at": 2000
	    }
	  }
	}`)

	snapshot, err := ParseUsage(payload, time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("ParseUsage() error = %v", err)
	}
	if snapshot.FiveHour != nil || snapshot.Weekly != nil {
		t.Fatalf("unsupported duration was mislabeled: %#v", snapshot)
	}
}

func TestFetchUsesCodexAuthAndDoesNotRequireAccountID(t *testing.T) {
	codexHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"tokens":{"access_token":"secret-token"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	fake := &fakeHTTPClient{response: `{"rate_limit":{"primary_window":{"used_percent":1}}}`}
	client := &Client{
		CodexHome: codexHome,
		Endpoint:  "https://example.test/usage",
		HTTP:      fake,
	}

	if _, err := client.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if fake.authorization != "Bearer secret-token" {
		t.Fatalf("authorization header = %q", fake.authorization)
	}
}

func TestRefresherSnapshotReportsUnavailableBeforeFirstSuccess(t *testing.T) {
	refresher := NewRefresher(&Client{}, time.Minute)

	snapshot, errors := refresher.Snapshot()

	if len(errors) != 0 {
		t.Fatalf("errors = %#v, want none", errors)
	}
	if snapshot.Status != StatusUnavailable {
		t.Fatalf("status = %q, want %q", snapshot.Status, StatusUnavailable)
	}
	if snapshot.Fresh {
		t.Fatalf("fresh = true, want false")
	}
}

func TestRefresherRefreshDowngradesToStaleWhenCachedSnapshotExists(t *testing.T) {
	codexHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"tokens":{"access_token":"secret-token"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	client := &Client{
		CodexHome: codexHome,
		Endpoint:  "https://example.test/usage",
		HTTP: &sequenceHTTPClient{
			results: []httpResult{
				{response: `{"plan_type":"pro","rate_limit":{"primary_window":{"used_percent":12,"reset_at":2000},"secondary_window":{"used_percent":20,"reset_at":3000}}}`},
				{err: errors.New("boom")},
			},
		},
	}
	refresher := NewRefresher(client, time.Minute)

	refresher.Refresh(context.Background())
	online, _ := refresher.Snapshot()
	if online.Status != StatusOK {
		t.Fatalf("initial status = %q, want %q", online.Status, StatusOK)
	}
	if !online.Fresh {
		t.Fatalf("initial fresh = false, want true")
	}

	refresher.Refresh(context.Background())
	stale, gotErrors := refresher.Snapshot()
	if len(gotErrors) != 0 {
		t.Fatalf("errors = %#v, want none", gotErrors)
	}
	if stale.Status != StatusStale {
		t.Fatalf("stale status = %q, want %q", stale.Status, StatusStale)
	}
	if stale.Fresh {
		t.Fatalf("stale fresh = true, want false")
	}
	if stale.FiveHour == nil || stale.Weekly == nil {
		t.Fatalf("stale snapshot lost cached windows: %#v", stale)
	}
}

func TestRefresherRefreshReportsUnavailableWhenNoCachedSnapshotExists(t *testing.T) {
	codexHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte(`{"tokens":{"access_token":"secret-token"}}`), 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}

	refresher := NewRefresher(&Client{
		CodexHome: codexHome,
		Endpoint:  "https://example.test/usage",
		HTTP: &sequenceHTTPClient{
			results: []httpResult{{err: errors.New("boom")}},
		},
	}, time.Minute)

	refresher.Refresh(context.Background())
	snapshot, gotErrors := refresher.Snapshot()
	if len(gotErrors) != 0 {
		t.Fatalf("errors = %#v, want none", gotErrors)
	}
	if snapshot.Status != StatusUnavailable {
		t.Fatalf("status = %q, want %q", snapshot.Status, StatusUnavailable)
	}
	if snapshot.FiveHour != nil || snapshot.Weekly != nil {
		t.Fatalf("unavailable snapshot should not expose windows: %#v", snapshot)
	}
}

type fakeHTTPClient struct {
	response      string
	authorization string
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.authorization = req.Header.Get("Authorization")
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(f.response)),
		Header:     make(http.Header),
	}, nil
}

type httpResult struct {
	response   string
	statusCode int
	err        error
}

type sequenceHTTPClient struct {
	results []httpResult
	index   int
}

func (c *sequenceHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if len(c.results) == 0 {
		return nil, errors.New("no response queued")
	}
	result := c.results[c.index]
	if c.index < len(c.results)-1 {
		c.index++
	}
	if result.err != nil {
		return nil, result.err
	}
	statusCode := result.statusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(result.response)),
		Header:     make(http.Header),
	}, nil
}
