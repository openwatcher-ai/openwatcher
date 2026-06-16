package sessions

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"openwatcher/internal/codexcompact"
)

func TestSnapshotAtReadsThreadMetadataAndContextPressure(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1be"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T01:20:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5000000,"cached_input_tokens":4600000,"output_tokens":25000,"reasoning_output_tokens":12000,"total_tokens":5037000},"last_token_usage":{"input_tokens":160000,"cached_input_tokens":150000,"output_tokens":4000,"reasoning_output_tokens":1200,"total_tokens":165200},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-03T01:48:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":5080697,"cached_input_tokens":4690048,"output_tokens":29882,"reasoning_output_tokens":14594,"total_tokens":5110579},"last_token_usage":{"input_tokens":180000,"cached_input_tokens":170000,"output_tokens":5200,"reasoning_output_tokens":2024,"total_tokens":187224},"model_context_window":920000}}}`,
	})
	writeModelCatalog(t, filepath.Join(codexHome, "model_catalog.local.json"), `{
		"models": [
			{"id":"gpt-5.4","context_window":1000000,"auto_compact_token_limit":850000}
		]
	}`)
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "看下这个新设计的UI，结合当前项目代码事实，分析下有实现难度么",
		Model:           "gpt-5.4",
		ReasoningEffort: "xhigh",
		TokensUsedTotal: 5110579,
		UpdatedAt:       now.Add(-12 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(snapshot.Sessions))
	}

	session := snapshot.Sessions[0]
	if session.ThreadID != threadID {
		t.Fatalf("thread id = %q", session.ThreadID)
	}
	if session.Title == "" || session.Model != "gpt-5.4" || session.ReasoningEffort != "xhigh" {
		t.Fatalf("session metadata = %#v", session)
	}
	if session.TokensUsedTotal != 5110579 {
		t.Fatalf("tokens total = %d", session.TokensUsedTotal)
	}
	if session.ContextUsedTokens != 187224 || session.ContextWindow != 920000 {
		t.Fatalf("context fields = %#v", session)
	}
	if session.ContextPressurePercent != contextPressurePercent(187224, 920000) {
		t.Fatalf("context pressure = %d", session.ContextPressurePercent)
	}
	if session.ContextCompactThresholdTokens != 850000 {
		t.Fatalf("compact threshold tokens = %d", session.ContextCompactThresholdTokens)
	}
	if session.ContextCompactThresholdPercent != contextPressurePercent(850000, 920000) {
		t.Fatalf("compact threshold percent = %d", session.ContextCompactThresholdPercent)
	}
	if session.LastActiveAgoMinutes != 12 {
		t.Fatalf("last active = %d", session.LastActiveAgoMinutes)
	}
	if got, want := len(snapshot.Heatmap24h.Buckets), heatmapBucketCount; got != want {
		t.Fatalf("heatmap buckets = %d, want %d", got, want)
	}
}

func TestSnapshotAtAddsContextCompactionFromHookState(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("OPENWATCHER_COMPACT_STATE_DIR", t.TempDir())

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1be"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "compacting",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-1 * time.Minute),
		RolloutPath:     rolloutPath,
	})
	if err := codexcompact.HandleHook(strings.NewReader(`{"hook_event_name":"PreCompact","session_id":"`+threadID+`","turn_id":"turn-1","trigger":"auto","model":"gpt-5.4"}`), now.Add(-5*time.Second)); err != nil {
		t.Fatalf("HandleHook() error = %v", err)
	}

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	compaction := snapshot.Sessions[0].ContextCompaction
	if compaction == nil {
		t.Fatalf("ContextCompaction is nil")
	}
	if compaction.Trigger != "auto" || compaction.TurnID != "turn-1" {
		t.Fatalf("ContextCompaction = %#v", compaction)
	}
	if !compaction.StartedAt.Equal(now.Add(-5 * time.Second)) {
		t.Fatalf("StartedAt = %s", compaction.StartedAt)
	}
}

func TestSnapshotAtClearsContextCompactionAfterRemoteCompactFailure(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)
	t.Setenv("OPENWATCHER_COMPACT_STATE_DIR", t.TempDir())

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1bf"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-03T10:00:01Z","type":"event_msg","payload":{"type":"error","message":"Error running remote compact task"}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "compact failed",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-1 * time.Minute),
		RolloutPath:     rolloutPath,
	})
	if err := codexcompact.HandleHook(strings.NewReader(`{"hook_event_name":"PreCompact","session_id":"`+threadID+`","turn_id":"turn-1","trigger":"manual","model":"gpt-5.4"}`), now); err != nil {
		t.Fatalf("HandleHook() error = %v", err)
	}
	statePath, err := codexcompact.StatePath(threadID)
	if err != nil {
		t.Fatalf("StatePath() error = %v", err)
	}

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now.Add(2 * time.Second))
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if snapshot.Sessions[0].ContextCompaction != nil {
		t.Fatalf("ContextCompaction = %#v, want nil", snapshot.Sessions[0].ContextCompaction)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state file should be removed after failure, err=%v", err)
	}
}

func TestSnapshotAtReadsConfiguredModelCatalogPath(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	catalogPath := filepath.Join(home, "catalogs", "models.json")
	writeModelCatalog(t, catalogPath, `{
		"models": [
			{"id":"gpt-5.4","context_window":1000000,"auto_compact_token_limit":760000}
		]
	}`)
	if err := os.WriteFile(
		filepath.Join(codexHome, "config.toml"),
		[]byte("model_catalog_json = "+strconv.Quote(catalogPath)+"\n"),
		0o644,
	); err != nil {
		t.Fatalf("write config: %v", err)
	}

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e893e-f1ae-7342-90a1-6326c4fa9af6"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "configured catalog",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if got := snapshot.Sessions[0].ContextCompactThresholdTokens; got != 760000 {
		t.Fatalf("compact threshold tokens = %d, want 760000", got)
	}
}

func TestSnapshotAtPrefersResolvedThreadName(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e893e-f1ae-7342-90a1-6326c4fa9af6"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "sqlite title",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(ctx context.Context, gotCodexHome string, threadIDs []string) (map[string]string, error) {
		if gotCodexHome != codexHome {
			t.Fatalf("resolver codex home = %q, want %q", gotCodexHome, codexHome)
		}
		if len(threadIDs) != 1 || threadIDs[0] != threadID {
			t.Fatalf("resolver thread ids = %#v", threadIDs)
		}
		return map[string]string{threadID: "Codex 标题"}, nil
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if got := snapshot.Sessions[0].Title; got != "Codex 标题" {
		t.Fatalf("title = %q, want resolved name", got)
	}
}

func TestSnapshotAtFallsBackToStateTitleWhenResolvedNameBlank(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e893e-f1ae-7342-90a1-6326c4fa9af7"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "sqlite fallback",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return map[string]string{threadID: "   "}, nil
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if got := snapshot.Sessions[0].Title; got != "sqlite fallback" {
		t.Fatalf("title = %q, want sqlite fallback", got)
	}
}

func TestSnapshotAtFallsBackToStateTitleWhenResolverFails(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e893e-f1ae-7342-90a1-6326c4fa9af8"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "sqlite fallback",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, errors.New("boom")
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if got := snapshot.Sessions[0].Title; got != "sqlite fallback" {
		t.Fatalf("title = %q, want sqlite fallback", got)
	}
}

func TestSnapshotAtHeatmapCacheDoesNotDoubleCount(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T01:10:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":100},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-03T01:20:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":40,"output_tokens":40,"reasoning_output_tokens":20,"total_tokens":250},"last_token_usage":{"input_tokens":90,"cached_input_tokens":30,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":120},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "热力图测试",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		TokensUsedTotal: 250,
		UpdatedAt:       now.Add(-10 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	first, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("first SnapshotAt() error = %v", err)
	}
	second, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("second SnapshotAt() error = %v", err)
	}

	firstTotal := heatmapTotal(first.Heatmap24h.Buckets)
	secondTotal := heatmapTotal(second.Heatmap24h.Buckets)
	if firstTotal != 250 {
		t.Fatalf("first heatmap total = %d, want 250", firstTotal)
	}
	if secondTotal != firstTotal {
		t.Fatalf("second heatmap total = %d, want %d", secondTotal, firstTotal)
	}

	cachePath := filepath.Join(home, ".openwatcher", "cache", "session-metrics.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
}

func TestSelectTokenCountIncrementSkipsDuplicateSnapshot(t *testing.T) {
	previous := tokenTotals{
		InputTokens:           120,
		CachedInputTokens:     60,
		OutputTokens:          20,
		ReasoningOutputTokens: 10,
		TotalTokens:           150,
	}

	delta, ok, advance := selectTokenCountIncrement(previous, &previous)
	if ok || advance || !delta.isZero() {
		t.Fatalf("duplicate snapshot delta = %#v ok=%v advance=%v, want zero false false", delta, ok, advance)
	}
}

func TestSelectTokenCountIncrementIgnoresDroppedSnapshot(t *testing.T) {
	previous := tokenTotals{
		InputTokens:           310,
		CachedInputTokens:     62,
		OutputTokens:          24,
		ReasoningOutputTokens: 4,
		TotalTokens:           400,
	}
	current := tokenTotals{
		InputTokens:           10,
		CachedInputTokens:     2,
		OutputTokens:          4,
		ReasoningOutputTokens: 1,
		TotalTokens:           17,
	}

	delta, ok, advance := selectTokenCountIncrement(current, &previous)
	if ok || advance || !delta.isZero() {
		t.Fatalf("dropped snapshot delta = %#v ok=%v advance=%v, want zero false false", delta, ok, advance)
	}
}

func TestSnapshotAtHeatmapUsesCurrentShanghaiDay(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-02T15:50:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":80,"cached_input_tokens":10,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":90},"last_token_usage":{"input_tokens":80,"cached_input_tokens":10,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":90},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-02T16:10:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":200,"cached_input_tokens":50,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":230},"last_token_usage":{"input_tokens":120,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":140},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-03T01:10:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":260,"cached_input_tokens":70,"output_tokens":40,"reasoning_output_tokens":15,"total_tokens":300},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "当日热力图",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		TokensUsedTotal: 300,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}

	if snapshot.Heatmap24h.Timezone != "Asia/Shanghai" {
		t.Fatalf("timezone = %q", snapshot.Heatmap24h.Timezone)
	}
	if total := heatmapTotal(snapshot.Heatmap24h.Buckets); total != 210 {
		t.Fatalf("heatmap total = %d, want 210", total)
	}
	if firstBucket := snapshot.Heatmap24h.Buckets[0]; firstBucket.TotalTokens != 140 {
		t.Fatalf("first bucket total = %d, want 140", firstBucket.TotalTokens)
	}
	if secondBucket := snapshot.Heatmap24h.Buckets[1]; secondBucket.TotalTokens != 0 {
		t.Fatalf("second bucket total = %d, want 0", secondBucket.TotalTokens)
	}
	if snapshot.DailyUsage.TotalTokens != 210 {
		t.Fatalf("daily usage total = %d, want 210", snapshot.DailyUsage.TotalTokens)
	}
	if snapshot.DailyUsage.ActiveSessions != 1 {
		t.Fatalf("daily usage active sessions = %d, want 1", snapshot.DailyUsage.ActiveSessions)
	}
	if len(snapshot.DailyUsage.ModelTokenBreakdowns) != 1 {
		t.Fatalf("daily usage model breakdowns = %#v", snapshot.DailyUsage.ModelTokenBreakdowns)
	}
	if model := snapshot.DailyUsage.ModelTokenBreakdowns[0]; model.Model != "gpt-5.4" || model.TotalTokens != 210 {
		t.Fatalf("daily usage model = %#v, want gpt-5.4 total 210", model)
	}
}

func TestSnapshotAtHeatmapAvoidsInflatedInterleavedCumulativeSeries(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 4, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2-interleave-heatmap"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:05:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":200},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:06:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":6,"cached_input_tokens":2,"output_tokens":1,"reasoning_output_tokens":1,"total_tokens":10},"last_token_usage":{"input_tokens":6,"cached_input_tokens":2,"output_tokens":1,"reasoning_output_tokens":1,"total_tokens":10},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:07:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":180,"cached_input_tokens":60,"output_tokens":45,"reasoning_output_tokens":15,"total_tokens":300},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:08:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":180,"cached_input_tokens":60,"output_tokens":45,"reasoning_output_tokens":15,"total_tokens":300},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "交替累计热力图",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		TokensUsedTotal: 300,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}

	if total := heatmapTotal(snapshot.Heatmap24h.Buckets); total != 300 {
		t.Fatalf("heatmap total = %d, want 300", total)
	}
	if snapshot.DailyUsage.TotalTokens != 300 {
		t.Fatalf("daily usage total = %d, want 300", snapshot.DailyUsage.TotalTokens)
	}
}

func TestSnapshotAtDailyTrendUsesShanghaiCompleteDays(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2-trend"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T14:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":80,"cached_input_tokens":10,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":80},"last_token_usage":{"input_tokens":80,"cached_input_tokens":10,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":80},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-03T16:30:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":230,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":230},"last_token_usage":{"input_tokens":150,"cached_input_tokens":30,"output_tokens":20,"reasoning_output_tokens":5,"total_tokens":150},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T15:30:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":430,"cached_input_tokens":80,"output_tokens":60,"reasoning_output_tokens":20,"total_tokens":430},"last_token_usage":{"input_tokens":200,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":200},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T16:30:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":700,"cached_input_tokens":120,"output_tokens":90,"reasoning_output_tokens":30,"total_tokens":700},"last_token_usage":{"input_tokens":270,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":270},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "30 日趋势",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		TokensUsedTotal: 700,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAtWithOptions(now, SnapshotOptions{IncludeDailyTrend30d: true})
	if err != nil {
		t.Fatalf("SnapshotAtWithOptions() error = %v", err)
	}
	trend := snapshot.DailyTrend30d
	if trend == nil {
		t.Fatal("daily trend is nil")
	}
	if trend.Timezone != "Asia/Shanghai" {
		t.Fatalf("trend timezone = %q, want Asia/Shanghai", trend.Timezone)
	}
	if trend.StartDate != "2026-05-06" || trend.EndDate != "2026-06-04" {
		t.Fatalf("trend range = %s..%s", trend.StartDate, trend.EndDate)
	}
	if len(trend.Days) != dailyTrendDayCount {
		t.Fatalf("trend days = %d, want %d", len(trend.Days), dailyTrendDayCount)
	}
	if got := dailyTrendDayTotal(trend, "2026-06-03"); got != 80 {
		t.Fatalf("2026-06-03 total = %d, want 80", got)
	}
	if got := dailyTrendDayTotal(trend, "2026-06-04"); got != 350 {
		t.Fatalf("2026-06-04 total = %d, want 350", got)
	}
	if trend.PeakTokens != 350 {
		t.Fatalf("peak tokens = %d, want 350", trend.PeakTokens)
	}
	if trend.AverageTokens != 14 {
		t.Fatalf("average tokens = %d, want 14", trend.AverageTokens)
	}
}

func TestSnapshotAtDailyTrendAvoidsInflatedInterleavedCumulativeSeries(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 5, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019e8943-36f6-73b2-8a7a-c30c3ecc0ef2-interleave-trend"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:05:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":40,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":200},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:06:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":6,"cached_input_tokens":2,"output_tokens":1,"reasoning_output_tokens":1,"total_tokens":10},"last_token_usage":{"input_tokens":6,"cached_input_tokens":2,"output_tokens":1,"reasoning_output_tokens":1,"total_tokens":10},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:07:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":180,"cached_input_tokens":60,"output_tokens":45,"reasoning_output_tokens":15,"total_tokens":300},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
		`{"timestamp":"2026-06-04T01:08:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":180,"cached_input_tokens":60,"output_tokens":45,"reasoning_output_tokens":15,"total_tokens":300},"last_token_usage":{"input_tokens":60,"cached_input_tokens":20,"output_tokens":15,"reasoning_output_tokens":5,"total_tokens":100},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "交替累计趋势",
		Model:           "gpt-5.4",
		ReasoningEffort: "high",
		TokensUsedTotal: 300,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAtWithOptions(now, SnapshotOptions{IncludeDailyTrend30d: true})
	if err != nil {
		t.Fatalf("SnapshotAtWithOptions() error = %v", err)
	}
	trend := snapshot.DailyTrend30d
	if trend == nil {
		t.Fatal("daily trend is nil")
	}
	if got := dailyTrendDayTotal(trend, "2026-06-04"); got != 300 {
		t.Fatalf("2026-06-04 total = %d, want 300", got)
	}
	if trend.PeakTokens != 300 {
		t.Fatalf("peak tokens = %d, want 300", trend.PeakTokens)
	}
}

func TestSnapshotAtFallsBackToStateGlobWhenState5Missing(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	threadID := "019e893e-f1ae-7342-90a1-6326c4fa9af6"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_9.sqlite"), stateThreadRow{
		ThreadID:        threadID,
		Title:           "fallback",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     rolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(snapshot.Sessions))
	}
}

func TestSnapshotAtPrefersSqliteSubdirStateDB(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(filepath.Join(codexHome, "sqlite"), 0o755); err != nil {
		t.Fatalf("mkdir codex sqlite home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	oldThreadID := "019e893e-f1ae-7342-90a1-6326c4fa9af6"
	oldRolloutPath := writeRollout(t, codexHome, oldThreadID, []string{
		`{"timestamp":"2026-06-03T09:50:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":80,"cached_input_tokens":20,"output_tokens":10,"reasoning_output_tokens":5,"total_tokens":90},"last_token_usage":{"input_tokens":40,"cached_input_tokens":10,"output_tokens":5,"reasoning_output_tokens":5,"total_tokens":50},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID:        oldThreadID,
		Title:           "old root state",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 90,
		UpdatedAt:       now.Add(-10 * time.Minute),
		RolloutPath:     oldRolloutPath,
	})

	newThreadID := "019e893e-f1ae-7342-90a1-6326c4fa9af7"
	newRolloutPath := writeRollout(t, codexHome, newThreadID, []string{
		`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "sqlite", "state_5.sqlite"), stateThreadRow{
		ThreadID:        newThreadID,
		Title:           "new sqlite state",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		TokensUsedTotal: 150,
		UpdatedAt:       now.Add(-5 * time.Minute),
		RolloutPath:     newRolloutPath,
	})

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}
	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(snapshot.Sessions))
	}
	if got := snapshot.Sessions[0].ThreadID; got != newThreadID {
		t.Fatalf("thread id = %q, want sqlite subdir thread", got)
	}
}

func TestResolveStateDBPathFallsBackWhenSqliteSubdirMissing(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}

	statePath := filepath.Join(codexHome, "state_5.sqlite")
	writeStateDB(t, statePath, stateThreadRow{
		ThreadID:        "019e893e-f1ae-7342-90a1-6326c4fa9af6",
		Title:           "legacy state",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		UpdatedAt:       time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
		RolloutPath:     "rollout.jsonl",
	})

	got, err := resolveStateDBPath(codexHome)
	if err != nil {
		t.Fatalf("resolveStateDBPath() error = %v", err)
	}
	if got != statePath {
		t.Fatalf("state path = %q, want %q", got, statePath)
	}
}

func TestResolveStateDBPathSkipsUnusableSqliteSubdirStateDB(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	sqliteDir := filepath.Join(codexHome, "sqlite")
	if err := os.MkdirAll(sqliteDir, 0o755); err != nil {
		t.Fatalf("mkdir codex sqlite home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sqliteDir, "state_5.sqlite"), []byte("not sqlite"), 0o644); err != nil {
		t.Fatalf("write invalid sqlite state: %v", err)
	}

	legacyPath := filepath.Join(codexHome, "state_5.sqlite")
	writeStateDB(t, legacyPath, stateThreadRow{
		ThreadID:        "019e893e-f1ae-7342-90a1-6326c4fa9af6",
		Title:           "legacy state",
		Model:           "gpt-5.4",
		ReasoningEffort: "medium",
		UpdatedAt:       time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC),
		RolloutPath:     "rollout.jsonl",
	})

	got, err := resolveStateDBPath(codexHome)
	if err != nil {
		t.Fatalf("resolveStateDBPath() error = %v", err)
	}
	if got != legacyPath {
		t.Fatalf("state path = %q, want %q", got, legacyPath)
	}
}

func TestSnapshotAtReturnsMostRecentSessionsEvenWhenOlderThanWindow(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	var rows []stateThreadRow
	for i := 0; i < 6; i++ {
		threadID := "019e893e-f1ae-7342-90a1-6326c4fa9b0" + strconv.Itoa(i)
		rolloutPath := writeRollout(t, codexHome, threadID, []string{
			`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
		})
		rows = append(rows, stateThreadRow{
			ThreadID:        threadID,
			Title:           "session-" + strconv.Itoa(i),
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TokensUsedTotal: 150,
			UpdatedAt:       now.Add(-time.Duration(40+i) * time.Minute),
			RolloutPath:     rolloutPath,
		})
	}
	writeStateDBRows(t, filepath.Join(codexHome, "state_5.sqlite"), rows)

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 5 {
		t.Fatalf("sessions len = %d, want 5", len(snapshot.Sessions))
	}
	for i, session := range snapshot.Sessions {
		wantTitle := "session-" + strconv.Itoa(i)
		if session.Title != wantTitle {
			t.Fatalf("session[%d].Title = %q, want %q", i, session.Title, wantTitle)
		}
	}
}

func TestSnapshotAtFillsDisplayLimitAfterFilteringSessionsWithoutContext(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	invalidIndex := 3
	var rows []stateThreadRow
	for i := 0; i < 6; i++ {
		threadID := "019e893e-f1ae-7342-90a1-6326c4fa9d0" + strconv.Itoa(i)
		lines := []string{
			`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
		}
		if i == invalidIndex {
			lines = []string{
				`{"timestamp":"2026-06-03T09:56:00Z","type":"response_item","payload":{"type":"message"}}`,
			}
		}
		rolloutPath := writeRollout(t, codexHome, threadID, lines)
		rows = append(rows, stateThreadRow{
			ThreadID:        threadID,
			Title:           "session-" + strconv.Itoa(i),
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TokensUsedTotal: 150,
			UpdatedAt:       now.Add(-time.Duration(i) * time.Minute),
			RolloutPath:     rolloutPath,
		})
	}
	writeStateDBRows(t, filepath.Join(codexHome, "state_5.sqlite"), rows)

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 5 {
		t.Fatalf("sessions len = %d, want 5", len(snapshot.Sessions))
	}
	wantTitles := []string{"session-0", "session-1", "session-2", "session-4", "session-5"}
	for i, session := range snapshot.Sessions {
		if session.Title != wantTitles[i] {
			t.Fatalf("session[%d].Title = %q, want %q", i, session.Title, wantTitles[i])
		}
	}
}

func TestSnapshotAtFiltersChildThreadsAndReturnsFivePrimarySessions(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	now := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	type threadSpec struct {
		threadID string
		title    string
		offset   time.Duration
	}
	specs := []threadSpec{
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c00", title: "main-0", offset: 1 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c01", title: "child-0", offset: 2 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c02", title: "child-1", offset: 3 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c03", title: "main-1", offset: 4 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c04", title: "main-2", offset: 5 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c05", title: "main-3", offset: 6 * time.Minute},
		{threadID: "019e893e-f1ae-7342-90a1-6326c4fa9c06", title: "main-4", offset: 7 * time.Minute},
	}

	rows := make([]stateThreadRow, 0, len(specs))
	for _, spec := range specs {
		rolloutPath := writeRollout(t, codexHome, spec.threadID, []string{
			`{"timestamp":"2026-06-03T09:55:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
		})
		rows = append(rows, stateThreadRow{
			ThreadID:        spec.threadID,
			Title:           spec.title,
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TokensUsedTotal: 150,
			UpdatedAt:       now.Add(-spec.offset),
			RolloutPath:     rolloutPath,
		})
	}
	statePath := filepath.Join(codexHome, "state_5.sqlite")
	writeStateDBRows(t, statePath, rows)
	writeThreadSpawnEdges(t, statePath, []stateThreadSpawnEdge{
		{
			ParentThreadID: "019e893e-f1ae-7342-90a1-6326c4fa9c00",
			ChildThreadID:  "019e893e-f1ae-7342-90a1-6326c4fa9c01",
			Status:         "open",
		},
		{
			ParentThreadID: "019e893e-f1ae-7342-90a1-6326c4fa9c00",
			ChildThreadID:  "019e893e-f1ae-7342-90a1-6326c4fa9c02",
			Status:         "closed",
		},
	})

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}

	snapshot, err := scanner.SnapshotAt(now)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 5 {
		t.Fatalf("sessions len = %d, want 5", len(snapshot.Sessions))
	}
	wantTitles := []string{"main-0", "main-1", "main-2", "main-3", "main-4"}
	for i, session := range snapshot.Sessions {
		if session.Title != wantTitles[i] {
			t.Fatalf("session[%d].Title = %q, want %q", i, session.Title, wantTitles[i])
		}
	}
}

func TestSnapshotAtRetainsActiveSessionContextsAcrossShanghaiMidnight(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	location := time.FixedZone("CST", 8*3600)
	beforeMidnight := time.Date(2026, 6, 4, 23, 58, 0, 0, location)
	afterMidnight := time.Date(2026, 6, 5, 0, 5, 0, 0, location)

	var rows []stateThreadRow
	for i := 0; i < 5; i++ {
		threadID := "019e9350-f1ae-7342-90a1-6326c4fa9c0" + strconv.Itoa(i)
		updatedAt := beforeMidnight.Add(-time.Duration(i) * time.Minute)
		rolloutPath := writeRollout(t, codexHome, threadID, []string{
			`{"timestamp":"2026-06-04T15:52:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":120,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":150},"last_token_usage":{"input_tokens":60,"cached_input_tokens":10,"output_tokens":20,"reasoning_output_tokens":10,"total_tokens":90},"model_context_window":920000}}}`,
		})
		rows = append(rows, stateThreadRow{
			ThreadID:        threadID,
			Title:           "session-" + strconv.Itoa(i),
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TokensUsedTotal: 150,
			UpdatedAt:       updatedAt,
			RolloutPath:     rolloutPath,
		})
	}
	writeStateDBRows(t, filepath.Join(codexHome, "state_5.sqlite"), rows)

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}

	firstSnapshot, err := scanner.SnapshotAt(beforeMidnight)
	if err != nil {
		t.Fatalf("first SnapshotAt() error = %v", err)
	}
	if len(firstSnapshot.Sessions) != 5 {
		t.Fatalf("first sessions len = %d, want 5", len(firstSnapshot.Sessions))
	}

	secondSnapshot, err := scanner.SnapshotAt(afterMidnight)
	if err != nil {
		t.Fatalf("second SnapshotAt() error = %v", err)
	}
	if len(secondSnapshot.Sessions) != 5 {
		t.Fatalf("second sessions len = %d, want 5", len(secondSnapshot.Sessions))
	}
	for i, session := range secondSnapshot.Sessions {
		wantTitle := "session-" + strconv.Itoa(i)
		if session.Title != wantTitle {
			t.Fatalf("second session[%d].Title = %q, want %q", i, session.Title, wantTitle)
		}
		if session.ContextWindow != 920000 {
			t.Fatalf("second session[%d].ContextWindow = %d, want 920000", i, session.ContextWindow)
		}
	}
	if secondSnapshot.DailyUsage.ActiveSessions != 0 {
		t.Fatalf("daily active sessions after midnight = %d, want 0", secondSnapshot.DailyUsage.ActiveSessions)
	}
}

func TestSnapshotAtBackfillsActiveSessionContextsWhenTodayHasNoRollouts(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	location := time.FixedZone("CST", 8*3600)
	afterMidnight := time.Date(2026, 6, 5, 0, 5, 0, 0, location)

	var rows []stateThreadRow
	for i := 0; i < 5; i++ {
		threadID := "019e9351-f1ae-7342-90a1-6326c4fa9d0" + strconv.Itoa(i)
		updatedAt := afterMidnight.Add(-time.Duration(10+i) * time.Minute)
		rolloutPath := writeRollout(t, codexHome, threadID, []string{
			`{"timestamp":"2026-06-04T15:52:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":220,"cached_input_tokens":40,"output_tokens":50,"reasoning_output_tokens":20,"total_tokens":290},"last_token_usage":{"input_tokens":80,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":10,"total_tokens":140},"model_context_window":920000}}}`,
		})
		rows = append(rows, stateThreadRow{
			ThreadID:        threadID,
			Title:           "backfill-" + strconv.Itoa(i),
			Model:           "gpt-5.4",
			ReasoningEffort: "medium",
			TokensUsedTotal: 290,
			UpdatedAt:       updatedAt,
			RolloutPath:     rolloutPath,
		})
	}
	writeStateDBRows(t, filepath.Join(codexHome, "state_5.sqlite"), rows)

	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		return nil, nil
	}

	snapshot, err := scanner.SnapshotAt(afterMidnight)
	if err != nil {
		t.Fatalf("SnapshotAt() error = %v", err)
	}
	if len(snapshot.Sessions) != 5 {
		t.Fatalf("sessions len = %d, want 5", len(snapshot.Sessions))
	}
	for i, session := range snapshot.Sessions {
		wantTitle := "backfill-" + strconv.Itoa(i)
		if session.Title != wantTitle {
			t.Fatalf("session[%d].Title = %q, want %q", i, session.Title, wantTitle)
		}
		if session.ContextUsedTokens != 140 {
			t.Fatalf("session[%d].ContextUsedTokens = %d, want 140", i, session.ContextUsedTokens)
		}
	}
	if snapshot.DailyUsage.ActiveSessions != 0 {
		t.Fatalf("daily active sessions after midnight = %d, want 0", snapshot.DailyUsage.ActiveSessions)
	}
}

func TestRolloutPathForThreadReadsOnlyNonArchivedStatePath(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	t.Setenv("HOME", home)

	activeThreadID := "019e8b13-dd60-7aa2-b20b-e060b280e1be"
	archivedThreadID := "019e8b13-dd60-7aa2-b20b-e060b280e1bf"
	activeRolloutPath := writeRollout(t, codexHome, activeThreadID, []string{})
	archivedRolloutPath := writeRollout(t, codexHome, archivedThreadID, []string{})
	writeStateDBRows(t, filepath.Join(codexHome, "state_5.sqlite"), []stateThreadRow{
		{
			ThreadID:    activeThreadID,
			Title:       "active",
			UpdatedAt:   time.Now(),
			RolloutPath: activeRolloutPath,
		},
		{
			ThreadID:    archivedThreadID,
			Title:       "archived",
			UpdatedAt:   time.Now(),
			RolloutPath: archivedRolloutPath,
			Archived:    true,
		},
	})

	scanner := NewScanner(codexHome, 5)
	got, err := scanner.RolloutPathForThread(activeThreadID)
	if err != nil {
		t.Fatalf("RolloutPathForThread(active) error = %v", err)
	}
	if got != filepath.Clean(activeRolloutPath) {
		t.Fatalf("active rollout path = %q, want %q", got, activeRolloutPath)
	}

	if _, err := scanner.RolloutPathForThread(archivedThreadID); err != ErrThreadNotFound {
		t.Fatalf("archived thread error = %v, want ErrThreadNotFound", err)
	}
	if _, err := scanner.RolloutPathForThread("missing-thread"); err != ErrThreadNotFound {
		t.Fatalf("missing thread error = %v, want ErrThreadNotFound", err)
	}
}

func TestAgentMessageReadersFilterTrimAndTruncate(t *testing.T) {
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1c0"
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	longText := strings.Repeat("界", agentMessageMaxRunes+5)
	lines := []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"user_message","message":"用户消息"}}`,
		`{"timestamp":"2026-06-04T01:00:01Z","type":"response_item","payload":{"type":"tool_output","message":"工具输出"}}`,
		`{"timestamp":"2026-06-04T01:00:02Z","type":"event_msg","payload":{"type":"agent_message","message":"  第一条  "}}`,
		`{"timestamp":"2026-06-04T01:00:03Z","type":"event_msg","payload":{"type":"agent_message","message":"   "}}`,
		`{"timestamp":"2026-06-04T01:00:04Z","type":"event_msg","payload":{"type":"agent_message","message":"` + longText + `"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	latest, offset, ok, err := LatestAgentMessage(path, threadID)
	if err != nil {
		t.Fatalf("LatestAgentMessage() error = %v", err)
	}
	if !ok {
		t.Fatal("LatestAgentMessage() did not find message")
	}
	if !latest.Truncated || len([]rune(latest.Text)) != agentMessageMaxRunes {
		t.Fatalf("latest truncation = %#v", latest)
	}
	if latest.ThreadID != threadID || latest.Type != "agent_message" || latest.EventID == "" {
		t.Fatalf("latest metadata = %#v", latest)
	}

	appended := `{"timestamp":"2026-06-04T01:00:05Z","type":"event_msg","payload":{"type":"agent_message","message":"  新增  "}}`
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open rollout append: %v", err)
	}
	if _, err := file.WriteString(appended + "\n"); err != nil {
		_ = file.Close()
		t.Fatalf("append rollout: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close rollout: %v", err)
	}

	messages, nextOffset, err := ReadAgentMessagesFromOffset(path, threadID, offset)
	if err != nil {
		t.Fatalf("ReadAgentMessagesFromOffset() error = %v", err)
	}
	if nextOffset <= offset {
		t.Fatalf("next offset = %d, previous %d", nextOffset, offset)
	}
	if len(messages) != 1 || messages[0].Text != "新增" {
		t.Fatalf("tailed messages = %#v", messages)
	}
}

func TestLatestAgentMessageFindsMessageBeforeLargeToolOutputs(t *testing.T) {
	threadID := "019ea2c5-40e1-7c92-b59b-6d72725947d5"
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	largeOutputA := strings.Repeat("A", 243005)
	largeOutputB := strings.Repeat("B", 237551)
	lines := []string{
		`{"timestamp":"2026-06-07T16:09:32.512Z","type":"event_msg","payload":{"type":"agent_message","message":"桌面应用已经用临时配置跑起来了，接下来我直接在真实窗口里逐页点过去并截 5 张图。"}}`,
		`{"timestamp":"2026-06-07T16:09:34.437Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-1","output":"` + largeOutputA + `"}}`,
		`{"timestamp":"2026-06-07T16:09:49.469Z","type":"response_item","payload":{"type":"function_call_output","call_id":"call-2","output":"` + largeOutputB + `"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	latest, _, ok, err := LatestAgentMessage(path, threadID)
	if err != nil {
		t.Fatalf("LatestAgentMessage() error = %v", err)
	}
	if !ok {
		t.Fatal("LatestAgentMessage() did not find message before large tool outputs")
	}
	if latest.Text != "桌面应用已经用临时配置跑起来了，接下来我直接在真实窗口里逐页点过去并截 5 张图。" {
		t.Fatalf("latest text = %q", latest.Text)
	}
	if latest.CreatedAt != "2026-06-07T16:09:32.512Z" {
		t.Fatalf("latest createdAt = %q", latest.CreatedAt)
	}
}

func TestRuntimeStateRecoversRunningToolPhase(t *testing.T) {
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1c1"
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	lines := []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:00:01Z","type":"response_item","payload":{"type":"reasoning"}}`,
		`{"timestamp":"2026-06-04T01:00:02Z","type":"response_item","payload":{"type":"function_call","call_id":"call-1","name":"exec_command","arguments":"不应转发"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	state, offset, err := LatestRuntimeState(path, threadID)
	if err != nil {
		t.Fatalf("LatestRuntimeState() error = %v", err)
	}
	if offset <= 0 {
		t.Fatalf("offset = %d", offset)
	}
	if !state.Running || state.Lifecycle != RuntimeLifecycleRunning || state.Phase != RuntimePhaseToolRunning {
		t.Fatalf("runtime state = %#v", state)
	}
	if state.ThreadID != threadID || state.TurnID != "turn-1" || state.StartedAt != "2026-06-04T01:00:00Z" || state.UpdatedAt != "2026-06-04T01:00:02Z" {
		t.Fatalf("runtime metadata = %#v", state)
	}
}

func TestRuntimeStateRecoversRunningTurnWhenTaskStartedIsBeforeMessageTail(t *testing.T) {
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1c1-tail"
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	var lines []string
	lines = append(lines,
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:00:01Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:10:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-2"}}`,
	)
	for index := 0; index < 3200; index++ {
		lines = append(lines, `{"timestamp":"2026-06-04T01:10:01Z","type":"response_item","payload":{"type":"noop","call_id":"`+strconv.Itoa(index)+`"}}`)
	}
	lines = append(lines, `{"timestamp":"2026-06-04T01:20:00Z","type":"event_msg","payload":{"type":"agent_message","phase":"commentary","message":"当前 turn 仍在输出"}}`)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	state, offset, err := LatestRuntimeState(path, threadID)
	if err != nil {
		t.Fatalf("LatestRuntimeState() error = %v", err)
	}
	if offset <= agentMessageReadChunkSize {
		t.Fatalf("test rollout size = %d, want larger than message tail window", offset)
	}
	if !state.Running || state.Lifecycle != RuntimeLifecycleRunning {
		t.Fatalf("runtime state = %#v", state)
	}
	if state.TurnID != "turn-2" || state.StartedAt != "2026-06-04T01:10:00Z" || state.Phase != RuntimePhaseAgentCommentary || state.UpdatedAt != "2026-06-04T01:20:00Z" {
		t.Fatalf("runtime metadata = %#v", state)
	}
}

func TestRuntimeStateIncrementalPhaseCompletedAndAborted(t *testing.T) {
	threadID := "019e8b13-dd60-7aa2-b20b-e060b280e1c2"
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	initial := []string{
		`{"timestamp":"2026-06-04T01:00:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:00:01Z","type":"response_item","payload":{"type":"reasoning"}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(initial, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}
	state, offset, machine, err := RecoverRuntimeState(path, threadID)
	if err != nil {
		t.Fatalf("RecoverRuntimeState() error = %v", err)
	}
	if state.Phase != RuntimePhaseReasoning || !state.Running {
		t.Fatalf("initial runtime state = %#v", state)
	}

	appended := []string{
		`{"timestamp":"2026-06-04T01:00:02Z","type":"event_msg","payload":{"type":"agent_message","phase":"commentary","message":"  进度消息  "}}`,
		`{"timestamp":"2026-06-04T01:00:03Z","type":"event_msg","payload":{"type":"agent_message","phase":"final_answer","message":"最终消息"}}`,
		`{"timestamp":"2026-06-04T01:00:04Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-06-04T01:00:05Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-2"}}`,
		`{"timestamp":"2026-06-04T01:00:06Z","type":"event_msg","payload":{"type":"turn_aborted","turn_id":"turn-2"}}`,
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("open rollout append: %v", err)
	}
	for _, line := range appended {
		if _, err := file.WriteString(line + "\n"); err != nil {
			_ = file.Close()
			t.Fatalf("append rollout: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close rollout: %v", err)
	}

	updates, nextOffset, err := ReadStreamUpdatesFromOffset(path, threadID, offset, machine)
	if err != nil {
		t.Fatalf("ReadStreamUpdatesFromOffset() error = %v", err)
	}
	if nextOffset <= offset {
		t.Fatalf("next offset = %d, previous %d", nextOffset, offset)
	}

	var states []RuntimeState
	var messages []AgentMessage
	for _, update := range updates {
		if update.RuntimeState != nil {
			states = append(states, *update.RuntimeState)
		}
		if update.AgentMessage != nil {
			messages = append(messages, *update.AgentMessage)
		}
	}
	if len(messages) != 2 || messages[0].Text != "进度消息" || messages[1].Text != "最终消息" {
		t.Fatalf("agent messages = %#v", messages)
	}
	if len(states) != 5 {
		t.Fatalf("states len = %d states=%#v", len(states), states)
	}
	if states[0].Phase != RuntimePhaseAgentCommentary || states[1].Phase != RuntimePhaseAgentFinal {
		t.Fatalf("message phases = %#v", states[:2])
	}
	if states[2].Lifecycle != RuntimeLifecycleCompleted || states[2].Running {
		t.Fatalf("completed state = %#v", states[2])
	}
	if states[3].TurnID != "turn-2" || !states[3].Running || states[3].Lifecycle != RuntimeLifecycleRunning {
		t.Fatalf("second turn running state = %#v", states[3])
	}
	if states[4].Lifecycle != RuntimeLifecycleAborted || states[4].Running {
		t.Fatalf("aborted state = %#v", states[4])
	}
}

type stateThreadRow struct {
	ThreadID        string
	Title           string
	Model           string
	ReasoningEffort string
	TokensUsedTotal int64
	UpdatedAt       time.Time
	RolloutPath     string
	Archived        bool
}

type stateThreadSpawnEdge struct {
	ParentThreadID string
	ChildThreadID  string
	Status         string
}

func writeStateDB(t *testing.T, path string, row stateThreadRow) {
	t.Helper()
	writeStateDBRows(t, path, []stateThreadRow{row})
}

func writeStateDBRows(t *testing.T, path string, rows []stateThreadRow) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	schema := `
CREATE TABLE threads (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    model TEXT,
    reasoning_effort TEXT,
    tokens_used INTEGER NOT NULL DEFAULT 0,
    updated_at INTEGER NOT NULL DEFAULT 0,
    updated_at_ms INTEGER,
    rollout_path TEXT NOT NULL DEFAULT '',
    archived INTEGER NOT NULL DEFAULT 0
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	for _, row := range rows {
		archived := 0
		if row.Archived {
			archived = 1
		}
		if _, err := db.Exec(
			`INSERT INTO threads (id, title, model, reasoning_effort, tokens_used, updated_at, updated_at_ms, rollout_path, archived) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			row.ThreadID,
			row.Title,
			row.Model,
			row.ReasoningEffort,
			row.TokensUsedTotal,
			row.UpdatedAt.Unix(),
			row.UpdatedAt.UnixMilli(),
			row.RolloutPath,
			archived,
		); err != nil {
			t.Fatalf("insert thread: %v", err)
		}
	}
}

func writeThreadSpawnEdges(t *testing.T, path string, edges []stateThreadSpawnEdge) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	schema := `
CREATE TABLE thread_spawn_edges (
    parent_thread_id TEXT NOT NULL,
    child_thread_id TEXT NOT NULL PRIMARY KEY,
    status TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create thread_spawn_edges schema: %v", err)
	}
	for _, edge := range edges {
		if _, err := db.Exec(
			`INSERT INTO thread_spawn_edges (parent_thread_id, child_thread_id, status) VALUES (?, ?, ?);`,
			edge.ParentThreadID,
			edge.ChildThreadID,
			edge.Status,
		); err != nil {
			t.Fatalf("insert thread_spawn_edge: %v", err)
		}
	}
}

func writeRollout(t *testing.T, codexHome, threadID string, lines []string) string {
	t.Helper()
	dir := filepath.Join(codexHome, "sessions", "2026", "06", "03")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}
	path := filepath.Join(dir, "rollout-2026-06-03T09-00-00-"+threadID+".jsonl")
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}
	return path
}

func writeModelCatalog(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir model catalog dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write model catalog: %v", err)
	}
}

func heatmapTotal(buckets []HeatmapBucket) int64 {
	total := int64(0)
	for _, bucket := range buckets {
		total += bucket.TotalTokens
	}
	return total
}

func dailyTrendDayTotal(trend *DailyTrend30dSnapshot, date string) int64 {
	for _, day := range trend.Days {
		if day.Date == date {
			return day.TotalTokens
		}
	}
	return -1
}
