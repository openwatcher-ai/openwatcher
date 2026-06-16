package sessions

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type metricsCache struct {
	Version         int                           `json:"version"`
	Timezone        string                        `json:"timezone"`
	DayStart        string                        `json:"dayStart,omitempty"`
	UpdatedAt       string                        `json:"updatedAt,omitempty"`
	RolloutCursors  map[string]rolloutCursor      `json:"rolloutCursors"`
	ThreadTotals    map[string]tokenTotals        `json:"threadTotals"`
	ThreadContexts  map[string]threadContextState `json:"threadContexts,omitempty"`
	HourBuckets     []cachedHeatmapBucket         `json:"hourBuckets"`
	ModelBuckets    []cachedDailyModelBucket      `json:"modelBuckets,omitempty"`
	BucketThreadIDs map[string][]string           `json:"bucketThreadIds,omitempty"`
}

type rolloutCursor struct {
	Offset        int64  `json:"offset"`
	LastTimestamp string `json:"lastTimestamp,omitempty"`
}

type threadContextState struct {
	ContextUsedTokens      int64  `json:"contextUsedTokens"`
	ContextWindow          int64  `json:"contextWindow"`
	ContextPressurePercent int    `json:"contextPressurePercent"`
	LastTimestamp          string `json:"lastTimestamp,omitempty"`
}

type cachedHeatmapBucket struct {
	HourStart             string `json:"hourStart"`
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	ActiveThreads         int    `json:"activeThreads"`
}

type cachedDailyModelBucket struct {
	Model                 string `json:"model"`
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
}

func resolveMetricsCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openwatcher", "cache", "session-metrics.json"), nil
}

func newMetricsCache(location *time.Location) metricsCache {
	zone := "Local"
	if location != nil && location.String() != "" {
		zone = location.String()
	}
	return metricsCache{
		Version:         metricsCacheVersion,
		Timezone:        zone,
		RolloutCursors:  map[string]rolloutCursor{},
		ThreadTotals:    map[string]tokenTotals{},
		ThreadContexts:  map[string]threadContextState{},
		HourBuckets:     []cachedHeatmapBucket{},
		ModelBuckets:    []cachedDailyModelBucket{},
		BucketThreadIDs: map[string][]string{},
	}
}

func loadMetricsCache(path string, location *time.Location) (metricsCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return metricsCache{}, err
	}

	cache := newMetricsCache(location)
	if err := json.Unmarshal(data, &cache); err != nil {
		return metricsCache{}, err
	}
	if cache.Version != metricsCacheVersion {
		return metricsCache{}, os.ErrInvalid
	}
	if cache.Timezone != newMetricsCache(location).Timezone {
		return metricsCache{}, os.ErrInvalid
	}
	if cache.RolloutCursors == nil {
		cache.RolloutCursors = map[string]rolloutCursor{}
	}
	if cache.ThreadTotals == nil {
		cache.ThreadTotals = map[string]tokenTotals{}
	}
	if cache.ThreadContexts == nil {
		cache.ThreadContexts = map[string]threadContextState{}
	}
	if cache.BucketThreadIDs == nil {
		cache.BucketThreadIDs = map[string][]string{}
	}
	if cache.ModelBuckets == nil {
		cache.ModelBuckets = []cachedDailyModelBucket{}
	}
	return cache, nil
}

func saveMetricsCache(path string, cache metricsCache) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Chmod(path, 0o600)
}

func refreshMetricsCache(path string, cache *metricsCache, rollouts []rolloutEntry, activeRows []threadRow, windowStart, now time.Time) {
	if cache.Version != metricsCacheVersion || cache.Timezone != newMetricsCache(now.Location()).Timezone {
		*cache = newMetricsCache(now.Location())
	}
	if cacheNeedsRebuild(*cache, rollouts) {
		*cache = newMetricsCache(now.Location())
	}
	resetDailyMetricsCacheIfNeeded(cache, windowStart)

	sort.Slice(rollouts, func(i, j int) bool {
		return rollouts[i].RolloutPath < rollouts[j].RolloutPath
	})

	for _, entry := range rollouts {
		scanRollout(entry, cache, windowStart)
	}
	backfillMissingThreadContexts(cache, activeRows)

	trimMetricsCache(cache, rollouts, activeRows, windowStart)
	cache.UpdatedAt = now.Format(time.RFC3339)
	_ = saveMetricsCache(path, *cache)
}

func resetDailyMetricsCacheIfNeeded(cache *metricsCache, windowStart time.Time) {
	dayStart := windowStart.Format(time.RFC3339)
	if cache.DayStart == dayStart {
		return
	}
	cache.DayStart = dayStart
	cache.HourBuckets = []cachedHeatmapBucket{}
	cache.ModelBuckets = []cachedDailyModelBucket{}
	cache.BucketThreadIDs = map[string][]string{}
}

func cacheNeedsRebuild(cache metricsCache, rollouts []rolloutEntry) bool {
	for _, entry := range rollouts {
		cursor, ok := cache.RolloutCursors[entry.RolloutPath]
		if !ok {
			continue
		}
		info, err := os.Stat(entry.RolloutPath)
		if err != nil {
			continue
		}
		if cursor.Offset > info.Size() {
			return true
		}
	}
	return false
}

func scanRollout(entry rolloutEntry, cache *metricsCache, windowStart time.Time) {
	file, err := os.Open(entry.RolloutPath)
	if err != nil {
		delete(cache.RolloutCursors, entry.RolloutPath)
		return
	}
	defer file.Close()

	cursor := cache.RolloutCursors[entry.RolloutPath]
	if cursor.Offset > 0 {
		if _, err := file.Seek(cursor.Offset, io.SeekStart); err != nil {
			cursor = rolloutCursor{}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				return
			}
		}
	}

	reader := bufio.NewReader(file)
	currentOffset := cursor.Offset
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			currentOffset += int64(len(line))
			processTokenCountLine(entry.ThreadID, entry.Model, line, cache, windowStart)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}
	}

	cache.RolloutCursors[entry.RolloutPath] = rolloutCursor{
		Offset:        currentOffset,
		LastTimestamp: cache.ThreadContexts[entry.ThreadID].LastTimestamp,
	}
}

func backfillMissingThreadContexts(cache *metricsCache, activeRows []threadRow) {
	for _, row := range activeRows {
		existing, ok := cache.ThreadContexts[row.ThreadID]
		if ok && existing.ContextWindow > 0 {
			continue
		}
		if row.RolloutPath == "" {
			continue
		}
		contextState, totals, cursor, ok := readLatestThreadContext(row.RolloutPath)
		if !ok {
			continue
		}
		cache.ThreadContexts[row.ThreadID] = contextState
		cache.ThreadTotals[row.ThreadID] = totals
		cache.RolloutCursors[row.RolloutPath] = cursor
	}
}

func readLatestThreadContext(path string) (threadContextState, tokenTotals, rolloutCursor, bool) {
	file, err := os.Open(path)
	if err != nil {
		return threadContextState{}, tokenTotals{}, rolloutCursor{}, false
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var (
		currentOffset int64
		found         bool
		lastContext   threadContextState
		lastTotals    tokenTotals
		lastCursor    rolloutCursor
	)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			currentOffset += int64(len(line))
			event, ok := parseTokenCountEvent(line)
			if ok {
				eventTime, parseErr := time.Parse(time.RFC3339Nano, event.Timestamp)
				if parseErr == nil {
					payload := event.Payload
					contextWindow := payload.Info.ModelContextWindow
					if contextWindow <= 0 {
						contextWindow = payload.ModelContextWindow
					}
					lastContext = threadContextState{
						ContextUsedTokens:      payload.Info.LastTokenUsage.TotalTokens,
						ContextWindow:          contextWindow,
						ContextPressurePercent: contextPressurePercent(payload.Info.LastTokenUsage.TotalTokens, contextWindow),
						LastTimestamp:          eventTime.Format(time.RFC3339Nano),
					}
					lastTotals = payload.Info.TotalTokenUsage
					lastCursor = rolloutCursor{
						Offset:        currentOffset,
						LastTimestamp: lastContext.LastTimestamp,
					}
					found = true
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return threadContextState{}, tokenTotals{}, rolloutCursor{}, false
		}
	}
	if !found {
		return threadContextState{}, tokenTotals{}, rolloutCursor{}, false
	}
	return lastContext, lastTotals, lastCursor, true
}

func processTokenCountLine(threadID string, model string, line []byte, cache *metricsCache, windowStart time.Time) {
	event, ok := parseTokenCountEvent(line)
	if !ok {
		return
	}

	eventTime, err := time.Parse(time.RFC3339Nano, event.Timestamp)
	if err != nil {
		return
	}

	payload := event.Payload
	contextWindow := payload.Info.ModelContextWindow
	if contextWindow <= 0 {
		contextWindow = payload.ModelContextWindow
	}

	cache.ThreadContexts[threadID] = threadContextState{
		ContextUsedTokens:      payload.Info.LastTokenUsage.TotalTokens,
		ContextWindow:          contextWindow,
		ContextPressurePercent: contextPressurePercent(payload.Info.LastTokenUsage.TotalTokens, contextWindow),
		LastTimestamp:          eventTime.Format(time.RFC3339Nano),
	}

	currentTotals := payload.Info.TotalTokenUsage
	previousTotals, hasPrevious := cache.ThreadTotals[threadID]
	delta, ok, advance := selectTokenCountIncrement(currentTotals, optionalPreviousTotals(previousTotals, hasPrevious))
	if advance {
		cache.ThreadTotals[threadID] = currentTotals
	}
	if !ok {
		return
	}
	if eventTime.In(windowStart.Location()).Truncate(time.Hour).Before(windowStart) {
		return
	}
	addTotalsToBucket(cache, threadID, model, eventTime, delta, windowStart)
}

func addTotalsToBucket(cache *metricsCache, threadID string, model string, eventTime time.Time, delta tokenTotals, windowStart time.Time) {
	if delta.TotalTokens <= 0 {
		return
	}

	hourStart := eventTime.In(windowStart.Location()).Truncate(time.Hour)
	if hourStart.Before(windowStart) {
		return
	}

	key := hourStart.Format(time.RFC3339)
	bucket := findOrCreateBucket(cache, key)
	bucket.InputTokens += delta.InputTokens
	bucket.CachedInputTokens += delta.CachedInputTokens
	bucket.OutputTokens += delta.OutputTokens
	bucket.ReasoningOutputTokens += delta.ReasoningOutputTokens
	bucket.TotalTokens += delta.TotalTokens

	if markThreadActive(cache, key, threadID) {
		bucket.ActiveThreads++
	}
	addTotalsToModelBucket(cache, model, delta)
}

func addTotalsToModelBucket(cache *metricsCache, model string, delta tokenTotals) {
	key := strings.ToLower(strings.TrimSpace(model))
	if key == "" {
		key = "unknown"
	}
	bucket := findOrCreateModelBucket(cache, key)
	bucket.InputTokens += delta.InputTokens
	bucket.CachedInputTokens += delta.CachedInputTokens
	bucket.OutputTokens += delta.OutputTokens
	bucket.ReasoningOutputTokens += delta.ReasoningOutputTokens
	bucket.TotalTokens += delta.TotalTokens
}

func findOrCreateModelBucket(cache *metricsCache, model string) *cachedDailyModelBucket {
	for index := range cache.ModelBuckets {
		if cache.ModelBuckets[index].Model == model {
			return &cache.ModelBuckets[index]
		}
	}
	cache.ModelBuckets = append(cache.ModelBuckets, cachedDailyModelBucket{Model: model})
	return &cache.ModelBuckets[len(cache.ModelBuckets)-1]
}

func findOrCreateBucket(cache *metricsCache, hourStart string) *cachedHeatmapBucket {
	for index := range cache.HourBuckets {
		if cache.HourBuckets[index].HourStart == hourStart {
			return &cache.HourBuckets[index]
		}
	}
	cache.HourBuckets = append(cache.HourBuckets, cachedHeatmapBucket{HourStart: hourStart})
	return &cache.HourBuckets[len(cache.HourBuckets)-1]
}

func markThreadActive(cache *metricsCache, bucketKey, threadID string) bool {
	activeThreads := cache.BucketThreadIDs[bucketKey]
	for _, existing := range activeThreads {
		if existing == threadID {
			return false
		}
	}
	cache.BucketThreadIDs[bucketKey] = append(activeThreads, threadID)
	return true
}

func trimMetricsCache(cache *metricsCache, rollouts []rolloutEntry, activeRows []threadRow, windowStart time.Time) {
	activeRollouts := make(map[string]rolloutEntry, len(rollouts)+len(activeRows))
	activeThreads := make(map[string]bool, len(rollouts)+len(activeRows))
	for _, entry := range rollouts {
		activeRollouts[entry.RolloutPath] = entry
		activeThreads[entry.ThreadID] = true
	}
	for _, row := range activeRows {
		activeThreads[row.ThreadID] = true
		if row.RolloutPath == "" {
			continue
		}
		activeRollouts[row.RolloutPath] = rolloutEntry{
			ThreadID:    row.ThreadID,
			Model:       row.Model,
			RolloutPath: row.RolloutPath,
		}
	}

	for path := range cache.RolloutCursors {
		if _, ok := activeRollouts[path]; !ok {
			delete(cache.RolloutCursors, path)
		}
	}
	for threadID := range cache.ThreadTotals {
		if !activeThreads[threadID] {
			delete(cache.ThreadTotals, threadID)
		}
	}
	for threadID := range cache.ThreadContexts {
		if !activeThreads[threadID] {
			delete(cache.ThreadContexts, threadID)
		}
	}

	filteredBuckets := make([]cachedHeatmapBucket, 0, heatmapBucketCount)
	for _, bucket := range cache.HourBuckets {
		hourStart, err := time.Parse(time.RFC3339, bucket.HourStart)
		if err != nil {
			continue
		}
		if hourStart.Before(windowStart) {
			delete(cache.BucketThreadIDs, bucket.HourStart)
			continue
		}
		filteredBuckets = append(filteredBuckets, bucket)
	}
	sort.Slice(filteredBuckets, func(i, j int) bool {
		return filteredBuckets[i].HourStart < filteredBuckets[j].HourStart
	})
	cache.HourBuckets = filteredBuckets

	for bucketKey := range cache.BucketThreadIDs {
		hourStart, err := time.Parse(time.RFC3339, bucketKey)
		if err != nil || hourStart.Before(windowStart) {
			delete(cache.BucketThreadIDs, bucketKey)
		}
	}

	sort.Slice(cache.ModelBuckets, func(i, j int) bool {
		return cache.ModelBuckets[i].TotalTokens > cache.ModelBuckets[j].TotalTokens
	})
}

func buildHeatmapSnapshot(cache metricsCache, now time.Time) Heatmap24hSnapshot {
	location := now.Location()
	bucketsByHour := make(map[string]cachedHeatmapBucket, len(cache.HourBuckets))
	for _, bucket := range cache.HourBuckets {
		bucketsByHour[bucket.HourStart] = bucket
	}

	windowStart := dayWindowStart(now)
	publicBuckets := make([]HeatmapBucket, 0, heatmapBucketCount)
	var (
		peakTokens int64
		peakStart  *time.Time
	)
	for index := 0; index < heatmapBucketCount; index++ {
		hourStart := windowStart.Add(time.Duration(index) * time.Hour)
		key := hourStart.Format(time.RFC3339)
		cached := bucketsByHour[key]
		bucket := HeatmapBucket{
			HourStart:             hourStart,
			InputTokens:           cached.InputTokens,
			CachedInputTokens:     cached.CachedInputTokens,
			OutputTokens:          cached.OutputTokens,
			ReasoningOutputTokens: cached.ReasoningOutputTokens,
			TotalTokens:           cached.TotalTokens,
			ActiveThreads:         cached.ActiveThreads,
		}
		if bucket.TotalTokens >= peakTokens && bucket.TotalTokens > 0 {
			peakTokens = bucket.TotalTokens
			copyHour := hourStart
			peakStart = &copyHour
		}
		publicBuckets = append(publicBuckets, bucket)
	}

	return Heatmap24hSnapshot{
		Timezone:      location.String(),
		GeneratedAt:   now,
		PeakHourStart: peakStart,
		Buckets:       publicBuckets,
	}
}

func buildDailyTokenUsage(cache metricsCache, now time.Time) DailyTokenUsage {
	usage := DailyTokenUsage{
		GeneratedAt:          now,
		ModelTokenBreakdowns: make([]DailyModelTokenUsage, 0, len(cache.ModelBuckets)),
	}
	activeThreads := map[string]bool{}
	for _, threadIDs := range cache.BucketThreadIDs {
		for _, threadID := range threadIDs {
			activeThreads[threadID] = true
		}
	}
	usage.ActiveSessions = len(activeThreads)
	for _, bucket := range cache.ModelBuckets {
		if bucket.TotalTokens <= 0 {
			continue
		}
		usage.InputTokens += bucket.InputTokens
		usage.CachedInputTokens += bucket.CachedInputTokens
		usage.OutputTokens += bucket.OutputTokens
		usage.ReasoningOutputTokens += bucket.ReasoningOutputTokens
		usage.TotalTokens += bucket.TotalTokens
		usage.ModelTokenBreakdowns = append(usage.ModelTokenBreakdowns, DailyModelTokenUsage{
			Model:                 bucket.Model,
			InputTokens:           bucket.InputTokens,
			CachedInputTokens:     bucket.CachedInputTokens,
			OutputTokens:          bucket.OutputTokens,
			ReasoningOutputTokens: bucket.ReasoningOutputTokens,
			TotalTokens:           bucket.TotalTokens,
		})
	}
	sort.Slice(usage.ModelTokenBreakdowns, func(i, j int) bool {
		return usage.ModelTokenBreakdowns[i].TotalTokens > usage.ModelTokenBreakdowns[j].TotalTokens
	})
	return usage
}

func dayWindowStart(now time.Time) time.Time {
	localNow := now.In(now.Location())
	return time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, localNow.Location())
}

func (totals tokenTotals) deltaFrom(previous tokenTotals) (tokenTotals, bool) {
	if totals.InputTokens < previous.InputTokens ||
		totals.CachedInputTokens < previous.CachedInputTokens ||
		totals.OutputTokens < previous.OutputTokens ||
		totals.ReasoningOutputTokens < previous.ReasoningOutputTokens ||
		totals.TotalTokens < previous.TotalTokens {
		return tokenTotals{}, false
	}
	return tokenTotals{
		InputTokens:           totals.InputTokens - previous.InputTokens,
		CachedInputTokens:     totals.CachedInputTokens - previous.CachedInputTokens,
		OutputTokens:          totals.OutputTokens - previous.OutputTokens,
		ReasoningOutputTokens: totals.ReasoningOutputTokens - previous.ReasoningOutputTokens,
		TotalTokens:           totals.TotalTokens - previous.TotalTokens,
	}, true
}

func (totals tokenTotals) isZero() bool {
	return totals.InputTokens == 0 &&
		totals.CachedInputTokens == 0 &&
		totals.OutputTokens == 0 &&
		totals.ReasoningOutputTokens == 0 &&
		totals.TotalTokens == 0
}

func selectTokenCountIncrement(current tokenTotals, previous *tokenTotals) (tokenTotals, bool, bool) {
	if previous != nil && current == *previous {
		return tokenTotals{}, false, false
	}
	if previous == nil {
		return current, current.TotalTokens > 0, true
	}

	delta, deltaOK := current.deltaFrom(*previous)
	if deltaOK {
		return delta, delta.TotalTokens > 0, true
	}
	return tokenTotals{}, false, false
}
