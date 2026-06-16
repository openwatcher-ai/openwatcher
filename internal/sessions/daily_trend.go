package sessions

import (
	"bufio"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

func buildDailyTrend30dSnapshot(rollouts []rolloutEntry, now time.Time) DailyTrend30dSnapshot {
	start, end := dailyTrendWindow(now)
	days := make([]DailyTrendDay, 0, dailyTrendDayCount)
	tokensByDate := make(map[string]int64, dailyTrendDayCount)
	modelTotals := make(map[string]tokenTotals, len(rollouts))
	for index := 0; index < dailyTrendDayCount; index++ {
		date := start.AddDate(0, 0, index).Format("2006-01-02")
		tokensByDate[date] = 0
		days = append(days, DailyTrendDay{Date: date})
	}

	for _, entry := range rollouts {
		scanRolloutDailyTrend(entry, start, end, tokensByDate, modelTotals)
	}

	var total int64
	var peak int64
	for index := range days {
		tokens := tokensByDate[days[index].Date]
		days[index].TotalTokens = tokens
		total += tokens
		if tokens > peak {
			peak = tokens
		}
	}

	return DailyTrend30dSnapshot{
		Timezone:             now.Location().String(),
		GeneratedAt:          now,
		StartDate:            start.Format("2006-01-02"),
		EndDate:              end.Format("2006-01-02"),
		TotalTokens:          total,
		AverageTokens:        total / int64(dailyTrendDayCount),
		PeakTokens:           peak,
		Days:                 days,
		ModelTokenBreakdowns: buildDailyTrendModelBreakdowns(modelTotals),
	}
}

func dailyTrendWindow(now time.Time) (time.Time, time.Time) {
	todayStart := dayWindowStart(now)
	end := todayStart.AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -(dailyTrendDayCount - 1))
	return start, end
}

func scanRolloutDailyTrend(entry rolloutEntry, start time.Time, end time.Time, tokensByDate map[string]int64, modelTotals map[string]tokenTotals) {
	file, err := os.Open(entry.RolloutPath)
	if err != nil {
		return
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var (
		previous    tokenTotals
		hasPrevious bool
	)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			event, ok := parseTokenCountEvent(line)
			if ok {
				eventTime, parseErr := time.Parse(time.RFC3339Nano, event.Timestamp)
				if parseErr == nil {
					current := event.Payload.Info.TotalTokenUsage
					delta, deltaOK, advance := selectTokenCountIncrement(current, optionalPreviousTotals(previous, hasPrevious))
					if advance {
						previous = current
						hasPrevious = true
					}
					if !deltaOK {
						continue
					}

					localDay := dayWindowStart(eventTime.In(start.Location()))
					if localDay.Before(start) || localDay.After(end) {
						continue
					}
					date := localDay.Format("2006-01-02")
					if _, exists := tokensByDate[date]; exists {
						tokensByDate[date] += delta.TotalTokens
						addTotalsToTrendModel(modelTotals, entry.Model, delta)
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}
	}
}

func optionalPreviousTotals(previous tokenTotals, hasPrevious bool) *tokenTotals {
	if !hasPrevious {
		return nil
	}
	return &previous
}

func addTotalsToTrendModel(modelTotals map[string]tokenTotals, model string, delta tokenTotals) {
	key := normalizeTrendModel(model)
	current := modelTotals[key]
	current.InputTokens += delta.InputTokens
	current.CachedInputTokens += delta.CachedInputTokens
	current.OutputTokens += delta.OutputTokens
	current.ReasoningOutputTokens += delta.ReasoningOutputTokens
	current.TotalTokens += delta.TotalTokens
	modelTotals[key] = current
}

func normalizeTrendModel(model string) string {
	value := strings.ToLower(strings.TrimSpace(model))
	if value == "" {
		return "unknown"
	}
	return value
}

func buildDailyTrendModelBreakdowns(modelTotals map[string]tokenTotals) []DailyModelTokenUsage {
	breakdowns := make([]DailyModelTokenUsage, 0, len(modelTotals))
	for model, totals := range modelTotals {
		if totals.TotalTokens <= 0 {
			continue
		}
		breakdowns = append(breakdowns, DailyModelTokenUsage{
			Model:                 model,
			InputTokens:           totals.InputTokens,
			CachedInputTokens:     totals.CachedInputTokens,
			OutputTokens:          totals.OutputTokens,
			ReasoningOutputTokens: totals.ReasoningOutputTokens,
			TotalTokens:           totals.TotalTokens,
		})
	}
	sort.Slice(breakdowns, func(i, j int) bool {
		return breakdowns[i].TotalTokens > breakdowns[j].TotalTokens
	})
	return breakdowns
}
