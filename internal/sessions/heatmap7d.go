package sessions

import (
	"bufio"
	"io"
	"os"
	"time"
)

func buildHeatmap7dSnapshot(rollouts []rolloutEntry, now time.Time) Heatmap7dSnapshot {
	start := dayWindowStart(now).AddDate(0, 0, -(heatmap7dDayCount - 1))
	dateToIndex := make(map[string]int, heatmap7dDayCount)
	days := make([]Heatmap7dDay, 0, heatmap7dDayCount)
	for index := 0; index < heatmap7dDayCount; index++ {
		date := start.AddDate(0, 0, index).Format("2006-01-02")
		dateToIndex[date] = index
		days = append(days, Heatmap7dDay{
			Date:  date,
			Hours: make([]int64, heatmapBucketCount),
		})
	}

	for _, entry := range rollouts {
		scanRolloutHeatmap7d(entry, start, now, dateToIndex, days)
	}

	var peakTokens int64
	for dayIndex := range days {
		var totalTokens int64
		for _, tokens := range days[dayIndex].Hours {
			totalTokens += tokens
			if tokens > peakTokens {
				peakTokens = tokens
			}
		}
		days[dayIndex].TotalTokens = totalTokens
	}

	return Heatmap7dSnapshot{
		Timezone:    now.Location().String(),
		GeneratedAt: now,
		StartDate:   days[0].Date,
		EndDate:     days[len(days)-1].Date,
		PeakTokens:  peakTokens,
		Days:        days,
	}
}

func scanRolloutHeatmap7d(
	entry rolloutEntry,
	start time.Time,
	now time.Time,
	dateToIndex map[string]int,
	days []Heatmap7dDay,
) {
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
					if !deltaOK || delta.TotalTokens <= 0 {
						continue
					}

					localTime := eventTime.In(start.Location())
					if localTime.Before(start) || localTime.After(now) {
						continue
					}
					date := dayWindowStart(localTime).Format("2006-01-02")
					dayIndex, exists := dateToIndex[date]
					if !exists {
						continue
					}
					hourIndex := localTime.Hour()
					if hourIndex < 0 || hourIndex >= heatmapBucketCount {
						continue
					}
					days[dayIndex].Hours[hourIndex] += delta.TotalTokens
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
