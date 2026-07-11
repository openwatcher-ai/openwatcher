package server

import (
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"openwatcher/internal/sessions"
)

func TestParseOpenAITextModelPricingHTMLParsesAstroRows(t *testing.T) {
	html := `
<astro-island component-export="TextTokenPricingTables" props="{&quot;tier&quot;:[0,&quot;standard&quot;],&quot;rows&quot;:[1,[[1,[[0,&quot;gpt-5.5 (&lt;272K context length)&quot;],[0,5],[0,0.5],[0,30]]],[1,[[0,&quot;gpt-5.5&quot;],[0,2.5],[0,0.25],[0,15]]],[1,[[0,&quot;gpt-5.3-codex&quot;],[0,1.75],[0,0.175],[0,14]]],[1,[[0,&quot;gpt-5-pro&quot;],[0,15],[0,null],[0,120]]]]]}"></astro-island>
`
	models, err := parseOpenAITextModelPricingHTML(html)
	if err != nil {
		t.Fatalf("parse pricing HTML: %v", err)
	}

	if got := models["gpt-5.5"]; got.InputPerMillion != 5 || got.CachedInputPerMillion != 0.5 || got.OutputPerMillion != 30 {
		t.Fatalf("gpt-5.5 pricing = %#v", got)
	}
	if got := models["gpt-5.3-codex"]; got.InputPerMillion != 1.75 || got.CachedInputPerMillion != 0.175 || got.OutputPerMillion != 14 {
		t.Fatalf("gpt-5.3-codex pricing = %#v", got)
	}
	if got := models["gpt-5-pro"]; got.InputPerMillion != 15 || got.CachedInputPerMillion != 0 || got.OutputPerMillion != 120 {
		t.Fatalf("gpt-5-pro pricing = %#v", got)
	}
}

func TestDailyPricingCacheFetchesAndReusesDailySnapshot(t *testing.T) {
	t.Setenv("OPENWATCHER_CACHE_DIR", t.TempDir())
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requests, 1)
		_, _ = w.Write([]byte(`<astro-island component-export="TextTokenPricingTables" props="{&quot;tier&quot;:[0,&quot;standard&quot;],&quot;rows&quot;:[1,[[1,[[0,&quot;gpt-5.4&quot;],[0,2.5],[0,0.25],[0,15]]]]]}"></astro-island>`))
	}))
	defer server.Close()

	cache := newDailyPricingCache()
	cache.client = server.Client()
	cache.sourceURL = server.URL
	now := time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC)

	first, err := cache.snapshotForDay(now)
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests after first snapshot = %d, want 1", got)
	}
	if first.Source != "openai-docs+built-in-fallback" || first.Models["gpt-5.4"].OutputPerMillion != 15 {
		t.Fatalf("first snapshot = %#v", first)
	}
	if got := first.Models["gpt-5.6-sol"]; got.InputPerMillion != 5 || got.CachedInputPerMillion != 0.5 || got.OutputPerMillion != 30 {
		t.Fatalf("supplemented gpt-5.6-sol pricing = %#v", got)
	}

	second, err := cache.snapshotForDay(now.Add(2 * time.Hour))
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("requests after cached snapshot = %d, want 1", got)
	}
	if second.FetchedAt != first.FetchedAt || second.SourceURL != server.URL {
		t.Fatalf("cached snapshot changed: first=%#v second=%#v", first, second)
	}
}

func TestDailyPricingCacheUsesFallbackWhenFetchFails(t *testing.T) {
	t.Setenv("OPENWATCHER_CACHE_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cache := newDailyPricingCache()
	cache.client = server.Client()
	cache.sourceURL = server.URL

	snapshot, err := cache.snapshotForDay(time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("snapshot with fallback: %v", err)
	}
	if snapshot.Source != "built-in-fallback" {
		t.Fatalf("snapshot source = %q", snapshot.Source)
	}
	if got := snapshot.Models["gpt-5.4"]; got.InputPerMillion != 2.5 || got.OutputPerMillion != 15 {
		t.Fatalf("fallback gpt-5.4 pricing = %#v", got)
	}
	if got := snapshot.Models["gpt-5.6-luna"]; got.InputPerMillion != 1 || got.CachedInputPerMillion != 0.1 || got.OutputPerMillion != 6 {
		t.Fatalf("fallback gpt-5.6-luna pricing = %#v", got)
	}
}

func TestSupplementModelPricingPreservesFetchedValues(t *testing.T) {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC)
	snapshot := supplementModelPricingSnapshot(modelPricingSnapshot{
		Source: "openai-docs",
		Models: map[string]modelPricing{
			"gpt-5.6-sol": {InputPerMillion: 6, CachedInputPerMillion: 0.6, OutputPerMillion: 36},
		},
	}, now)
	if got := snapshot.Models["gpt-5.6-sol"]; got.InputPerMillion != 6 || got.OutputPerMillion != 36 {
		t.Fatalf("fetched pricing was overwritten: %#v", got)
	}
	if _, ok := snapshot.Models["gpt-5.4-mini"]; !ok {
		t.Fatal("missing built-in pricing was not supplemented")
	}
	if snapshot.Source != "openai-docs+built-in-fallback" {
		t.Fatalf("snapshot source = %q", snapshot.Source)
	}
}

func TestEstimateDailyValueDoesNotDoubleCountReasoningOutput(t *testing.T) {
	value, ok := estimateDailyValueUSD([]sessions.DailyModelTokenUsage{{
		Model:                 "gpt-5.5",
		InputTokens:           104367446,
		CachedInputTokens:     101343104,
		OutputTokens:          323488,
		ReasoningOutputTokens: 119569,
		TotalTokens:           104690934,
	}}, map[string]modelPricing{
		"gpt-5.5": {
			InputPerMillion:       5,
			CachedInputPerMillion: 0.5,
			OutputPerMillion:      30,
		},
	})
	if !ok {
		t.Fatal("estimateDailyValueUSD did not use priced model")
	}
	want := 75.497902
	if math.Abs(value-want) > 0.000001 {
		t.Fatalf("estimated value = %.4f, want %.4f", value, want)
	}
}
