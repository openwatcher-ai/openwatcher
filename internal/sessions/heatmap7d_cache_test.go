package sessions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCachedHeatmap7dReusesHistoryAndRefreshesToday(t *testing.T) {
	location, err := time.LoadLocation(heatmapTimezoneName)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, location)
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	firstLine := `{"timestamp":"2026-06-05T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":100}}}}`
	if err := os.WriteFile(path, []byte(firstLine+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner("", 5)
	rollouts := []rolloutEntry{{ThreadID: "thread-1", Model: "gpt-5.4", RolloutPath: path}}
	first := scanner.buildCachedHeatmap7dSnapshot(rollouts, now, todayHeatmap(now, 50))
	if got := heatmap7dDayTotal(first, "2026-06-05"); got != 100 {
		t.Fatalf("first historical total = %d, want 100", got)
	}
	if got := heatmap7dDayTotal(first, "2026-06-07"); got != 50 {
		t.Fatalf("first today total = %d, want 50", got)
	}

	secondLine := `{"timestamp":"2026-06-05T01:05:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":200}}}}`
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(secondLine + "\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	second := scanner.buildCachedHeatmap7dSnapshot(rollouts, now.Add(time.Minute), todayHeatmap(now.Add(time.Minute), 70))
	if got := heatmap7dDayTotal(second, "2026-06-05"); got != 100 {
		t.Fatalf("cached historical total = %d, want 100", got)
	}
	if got := heatmap7dDayTotal(second, "2026-06-07"); got != 70 {
		t.Fatalf("updated today total = %d, want 70", got)
	}

	refreshed := scanner.buildCachedHeatmap7dSnapshot(rollouts, now.Add(heatmap7dCacheTTL), todayHeatmap(now.Add(heatmap7dCacheTTL), 80))
	if got := heatmap7dDayTotal(refreshed, "2026-06-05"); got != 200 {
		t.Fatalf("refreshed historical total = %d, want 200", got)
	}
	if got := heatmap7dDayTotal(refreshed, "2026-06-07"); got != 80 {
		t.Fatalf("refreshed today total = %d, want 80", got)
	}
}

func TestSnapshotOptionsSkipSessionsAvoidsNameResolution(t *testing.T) {
	home := t.TempDir()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	threadID := "019ea2c5-40e1-7c92-b59b-6d72725947d5-skip"
	rolloutPath := writeRollout(t, codexHome, threadID, []string{
		`{"timestamp":"2026-06-07T01:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":100},"last_token_usage":{"total_tokens":100},"model_context_window":920000}}}`,
	})
	writeStateDB(t, filepath.Join(codexHome, "state_5.sqlite"), stateThreadRow{
		ThreadID: threadID, Title: "private title", Model: "gpt-5.4", TokensUsedTotal: 100,
		UpdatedAt: now.Add(-time.Minute), RolloutPath: rolloutPath,
	})

	resolverCalls := 0
	scanner := NewScanner(codexHome, 5)
	scanner.resolveThreadNames = func(context.Context, string, []string) (map[string]string, error) {
		resolverCalls++
		return map[string]string{threadID: "resolved private title"}, nil
	}
	snapshot, err := scanner.SnapshotAtWithOptions(now, SnapshotOptions{SkipSessions: true})
	if err != nil {
		t.Fatal(err)
	}
	if resolverCalls != 0 || len(snapshot.Sessions) != 0 {
		t.Fatalf("skip sessions still resolved details: calls=%d sessions=%#v", resolverCalls, snapshot.Sessions)
	}
	if snapshot.DailyUsage.TotalTokens != 100 {
		t.Fatalf("daily usage total = %d, want 100", snapshot.DailyUsage.TotalTokens)
	}
}

func todayHeatmap(now time.Time, total int64) Heatmap24hSnapshot {
	hour := dayWindowStart(now).Add(10 * time.Hour)
	return Heatmap24hSnapshot{Timezone: now.Location().String(), GeneratedAt: now, Buckets: []HeatmapBucket{{HourStart: hour, TotalTokens: total}}}
}

func heatmap7dDayTotal(snapshot Heatmap7dSnapshot, date string) int64 {
	for _, day := range snapshot.Days {
		if strings.EqualFold(day.Date, date) {
			return day.TotalTokens
		}
	}
	return -1
}
