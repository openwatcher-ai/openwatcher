package widgetprefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"openwatcher/desktop-app/widget/internal/widgetvm"
)

const maxTrendCacheBytes = 256 << 10

type TrendStore interface {
	LoadTrend30d() *widgetvm.Trend30d
	SaveTrend30d(*widgetvm.Trend30d) error
}

type memoryTrendStore struct {
	mu    sync.Mutex
	trend *widgetvm.Trend30d
}

func NewMemoryTrendStore() TrendStore { return &memoryTrendStore{} }

func (s *memoryTrendStore) LoadTrend30d() *widgetvm.Trend30d {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneTrend(s.trend)
}

func (s *memoryTrendStore) SaveTrend30d(trend *widgetvm.Trend30d) error {
	if !validTrend(trend) {
		return os.ErrInvalid
	}
	s.mu.Lock()
	s.trend = cloneTrend(trend)
	s.mu.Unlock()
	return nil
}

type trendFileStore struct{ path string }

func NewTrendFileStore(path string) TrendStore { return &trendFileStore{path: path} }

func DefaultTrendPath(home string) string {
	return filepath.Join(home, ".openwatcher", "cache", "widget-trend-30d.json")
}

func (s *trendFileStore) LoadTrend30d() *widgetvm.Trend30d {
	info, err := os.Stat(s.path)
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > maxTrendCacheBytes {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil
	}
	var trend widgetvm.Trend30d
	if json.Unmarshal(data, &trend) != nil || !validTrend(&trend) {
		return nil
	}
	return cloneTrend(&trend)
}

func (s *trendFileStore) SaveTrend30d(trend *widgetvm.Trend30d) error {
	if !validTrend(trend) {
		return os.ErrInvalid
	}
	return writePrivateJSON(s.path, ".widget-trend-*", trend)
}

func validTrend(trend *widgetvm.Trend30d) bool {
	if trend == nil || len(trend.Days) != 30 || trend.TotalTokens < 0 || trend.AverageTokens < 0 || trend.PeakTokens < 0 {
		return false
	}
	start, startErr := time.Parse("2006-01-02", trend.StartDate)
	end, endErr := time.Parse("2006-01-02", trend.EndDate)
	if startErr != nil || endErr != nil || end.Before(start) {
		return false
	}
	if !start.AddDate(0, 0, len(trend.Days)-1).Equal(end) {
		return false
	}
	for index, day := range trend.Days {
		if day.Date != start.AddDate(0, 0, index).Format("2006-01-02") || day.TotalTokens < 0 {
			return false
		}
	}
	return true
}

func cloneTrend(input *widgetvm.Trend30d) *widgetvm.Trend30d {
	if input == nil {
		return nil
	}
	output := *input
	output.Days = append([]widgetvm.TrendDay(nil), input.Days...)
	return &output
}
