package widgetprefs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"openwatcher/desktop-app/widget/internal/widgetvm"
)

func TestTrendFileStoreRoundTripIsPrivateAndRejectsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "trend.json")
	store := NewTrendFileStore(path)
	want := sampleTrend()
	if err := store.SaveTrend30d(want); err != nil {
		t.Fatal(err)
	}
	got := store.LoadTrend30d()
	if got == nil || got.EndDate != want.EndDate || len(got.Days) != 30 {
		t.Fatalf("loaded trend = %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("%v %v", info, err)
	}
	if err := os.WriteFile(path, []byte(`{"endDate":"bad"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if got := store.LoadTrend30d(); got != nil {
		t.Fatalf("corrupt trend was loaded: %#v", got)
	}
}

func sampleTrend() *widgetvm.Trend30d {
	start := time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC)
	trend := &widgetvm.Trend30d{Timezone: "Asia/Shanghai", StartDate: "2026-06-11", EndDate: "2026-07-10"}
	for index := 0; index < 30; index++ {
		trend.Days = append(trend.Days, widgetvm.TrendDay{Date: start.AddDate(0, 0, index).Format("2006-01-02"), TotalTokens: int64(index + 1)})
		trend.TotalTokens += int64(index + 1)
		trend.PeakTokens = int64(index + 1)
	}
	trend.AverageTokens = trend.TotalTokens / 30
	return trend
}
