package sessions

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"strings"
	"time"

	"openwatcher/internal/config"
)

const (
	contextPressureBaselineTokens = int64(12000)
	heatmapBucketCount            = 24
	heatmap7dDayCount             = 7
	dailyTrendDayCount            = 30
	metricsCacheVersion           = 4
	heatmapTimezoneName           = "Asia/Shanghai"
	threadNameResolverTimeout     = 2 * time.Second
	activeSessionCandidateLimit   = 30
)

type Scanner struct {
	CodexHome          string
	Limit              int
	resolveThreadNames threadNameResolverFunc
}

type Snapshot struct {
	Heatmap24h    Heatmap24hSnapshot     `json:"heatmap24h"`
	Heatmap7d     Heatmap7dSnapshot      `json:"heatmap7d"`
	DailyUsage    DailyTokenUsage        `json:"dailyUsage"`
	DailyTrend30d *DailyTrend30dSnapshot `json:"dailyTrend30d,omitempty"`
	Sessions      []SessionSnapshot      `json:"sessions"`
}

type SnapshotOptions struct {
	IncludeDailyTrend30d bool
}

type SessionSnapshot struct {
	ThreadID                       string                     `json:"threadId"`
	Title                          string                     `json:"title"`
	UpdatedAt                      time.Time                  `json:"updatedAt"`
	Model                          string                     `json:"model"`
	ReasoningEffort                string                     `json:"reasoningEffort,omitempty"`
	TokensUsedTotal                int64                      `json:"tokensUsedTotal"`
	ContextUsedTokens              int64                      `json:"contextUsedTokens"`
	ContextWindow                  int64                      `json:"contextWindow"`
	ContextPressurePercent         int                        `json:"contextPressurePercent"`
	ContextCompactThresholdTokens  int64                      `json:"contextCompactThresholdTokens,omitempty"`
	ContextCompactThresholdPercent int                        `json:"contextCompactThresholdPercent,omitempty"`
	LastActiveAgoMinutes           int64                      `json:"lastActiveAgoMinutes"`
	ContextCompaction              *ContextCompactionSnapshot `json:"contextCompaction,omitempty"`
}

type ContextCompactionSnapshot struct {
	Trigger   string    `json:"trigger,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	TurnID    string    `json:"turnId,omitempty"`
}

type Heatmap24hSnapshot struct {
	Timezone      string          `json:"timezone"`
	GeneratedAt   time.Time       `json:"generatedAt"`
	PeakHourStart *time.Time      `json:"peakHourStart,omitempty"`
	Buckets       []HeatmapBucket `json:"buckets"`
}

type HeatmapBucket struct {
	HourStart             time.Time `json:"hourStart"`
	InputTokens           int64     `json:"inputTokens"`
	CachedInputTokens     int64     `json:"cachedInputTokens"`
	OutputTokens          int64     `json:"outputTokens"`
	ReasoningOutputTokens int64     `json:"reasoningOutputTokens"`
	TotalTokens           int64     `json:"totalTokens"`
	ActiveThreads         int       `json:"activeThreads"`
}

type Heatmap7dSnapshot struct {
	Timezone    string         `json:"timezone"`
	GeneratedAt time.Time      `json:"generatedAt"`
	StartDate   string         `json:"startDate"`
	EndDate     string         `json:"endDate"`
	PeakTokens  int64          `json:"peakTokens"`
	Days        []Heatmap7dDay `json:"days"`
}

type Heatmap7dDay struct {
	Date        string  `json:"date"`
	TotalTokens int64   `json:"totalTokens"`
	Hours       []int64 `json:"hours"`
}

type threadRow struct {
	ThreadID        string
	Title           string
	Model           string
	ReasoningEffort string
	TokensUsedTotal int64
	UpdatedAt       time.Time
	RolloutPath     string
}

type rolloutEntry struct {
	ThreadID    string
	Model       string
	RolloutPath string
}

type DailyTokenUsage struct {
	GeneratedAt           time.Time              `json:"generatedAt"`
	InputTokens           int64                  `json:"inputTokens"`
	CachedInputTokens     int64                  `json:"cachedInputTokens"`
	OutputTokens          int64                  `json:"outputTokens"`
	ReasoningOutputTokens int64                  `json:"reasoningOutputTokens"`
	TotalTokens           int64                  `json:"totalTokens"`
	ActiveSessions        int                    `json:"activeSessions"`
	ModelTokenBreakdowns  []DailyModelTokenUsage `json:"modelTokenBreakdowns,omitempty"`
}

type DailyModelTokenUsage struct {
	Model                 string `json:"model"`
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
}

type DailyTrend30dSnapshot struct {
	Timezone      string          `json:"timezone"`
	GeneratedAt   time.Time       `json:"generatedAt"`
	StartDate     string          `json:"startDate"`
	EndDate       string          `json:"endDate"`
	TotalTokens   int64           `json:"totalTokens"`
	AverageTokens int64           `json:"averageTokens"`
	PeakTokens    int64           `json:"peakTokens"`
	Days          []DailyTrendDay `json:"days"`

	ModelTokenBreakdowns []DailyModelTokenUsage `json:"-"`
}

type DailyTrendDay struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"totalTokens"`
}

type tokenTotals struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type tokenCountEvent struct {
	Timestamp string            `json:"timestamp"`
	Type      string            `json:"type"`
	Payload   tokenCountPayload `json:"payload"`
}

type tokenCountPayload struct {
	Type               string `json:"type"`
	ModelContextWindow int64  `json:"model_context_window"`
	Info               struct {
		ModelContextWindow int64       `json:"model_context_window"`
		TotalTokenUsage    tokenTotals `json:"total_token_usage"`
		LastTokenUsage     tokenTotals `json:"last_token_usage"`
	} `json:"info"`
}

func NewScanner(codexHome string, limit int) *Scanner {
	if limit <= 0 {
		limit = config.DefaultActiveSessionLimit
	}
	return &Scanner{
		CodexHome:          codexHome,
		Limit:              limit,
		resolveThreadNames: resolveThreadNamesFromAppServer,
	}
}

func (s *Scanner) ActiveSessions() ([]SessionSnapshot, error) {
	snapshot, err := s.SnapshotAt(time.Now())
	if err != nil {
		return nil, err
	}
	return snapshot.Sessions, nil
}

func (s *Scanner) Snapshot() (Snapshot, error) {
	return s.SnapshotAt(time.Now())
}

func (s *Scanner) SnapshotWithOptions(options SnapshotOptions) (Snapshot, error) {
	return s.SnapshotAtWithOptions(time.Now(), options)
}

func (s *Scanner) SnapshotAt(now time.Time) (Snapshot, error) {
	return s.SnapshotAtWithOptions(now, SnapshotOptions{})
}

func (s *Scanner) SnapshotAtWithOptions(now time.Time, options SnapshotOptions) (Snapshot, error) {
	codexHome, err := config.ResolveCodexHome(s.CodexHome)
	if err != nil {
		return Snapshot{}, err
	}

	statePath, err := resolveStateDBPath(codexHome)
	if err != nil {
		return Snapshot{}, err
	}

	db, err := openStateDB(statePath)
	if err != nil {
		return Snapshot{}, err
	}
	defer db.Close()

	heatmapLocation := loadHeatmapLocation()
	heatmapNow := now.In(heatmapLocation)
	heatmapWindowStart := dayWindowStart(heatmapNow)
	heatmap7dWindowStart := heatmapWindowStart.AddDate(0, 0, -(heatmap7dDayCount - 1))

	displayLimit := s.Limit
	candidateLimit := activeSessionCandidateLimitForDisplayLimit(displayLimit)
	activeThreads, err := loadActiveThreads(db, candidateLimit)
	if err != nil {
		return Snapshot{}, err
	}
	if err := s.applyResolvedThreadNames(codexHome, activeThreads); err != nil {
		log.Printf("sessions: thread name resolver failed: %v", err)
	}

	recentRollouts, err := loadRecentRollouts(db, heatmapWindowStart)
	if err != nil {
		return Snapshot{}, err
	}
	weeklyHeatmapRollouts, err := loadRecentRollouts(db, heatmap7dWindowStart)
	if err != nil {
		return Snapshot{}, err
	}
	var dailyTrend30d *DailyTrend30dSnapshot
	if options.IncludeDailyTrend30d {
		trendStart, _ := dailyTrendWindow(heatmapNow)
		trendRollouts, err := loadRecentRollouts(db, trendStart)
		if err != nil {
			return Snapshot{}, err
		}
		trend := buildDailyTrend30dSnapshot(trendRollouts, heatmapNow)
		dailyTrend30d = &trend
	}

	cachePath, err := resolveMetricsCachePath()
	if err != nil {
		return Snapshot{}, err
	}

	cache, cacheErr := loadMetricsCache(cachePath, heatmapLocation)
	if cacheErr != nil {
		cache = newMetricsCache(heatmapLocation)
	}
	refreshMetricsCache(cachePath, &cache, recentRollouts, activeThreads, heatmapWindowStart, heatmapNow)

	thresholds := loadModelCompactThresholds(codexHome)
	compactions := loadContextCompactionStates(now)

	sessionSnapshots := buildSessionSnapshots(activeThreads, cache.ThreadContexts, thresholds, compactions, now)
	if len(sessionSnapshots) > displayLimit {
		sessionSnapshots = sessionSnapshots[:displayLimit]
	}

	return Snapshot{
		Heatmap24h:    buildHeatmapSnapshot(cache, heatmapNow),
		Heatmap7d:     buildHeatmap7dSnapshot(weeklyHeatmapRollouts, heatmapNow),
		DailyUsage:    buildDailyTokenUsage(cache, heatmapNow),
		DailyTrend30d: dailyTrend30d,
		Sessions:      sessionSnapshots,
	}, nil
}

func activeSessionCandidateLimitForDisplayLimit(displayLimit int) int {
	if displayLimit <= 0 {
		displayLimit = config.DefaultActiveSessionLimit
	}
	return max(displayLimit, activeSessionCandidateLimit)
}

func buildSessionSnapshots(
	rows []threadRow,
	contexts map[string]threadContextState,
	thresholds modelCompactThresholds,
	compactions map[string]contextCompactionState,
	now time.Time,
) []SessionSnapshot {
	sessions := make([]SessionSnapshot, 0, len(rows))
	for _, row := range rows {
		context, ok := contexts[row.ThreadID]
		if !ok || context.ContextWindow <= 0 {
			continue
		}

		lastActiveAgo := int64(0)
		if delta := now.Sub(row.UpdatedAt); delta > 0 {
			lastActiveAgo = int64(delta / time.Minute)
		}

		compactThresholdTokens := thresholds.thresholdForModel(row.Model)
		compactThresholdPercent := 0
		if compactThresholdTokens > 0 {
			compactThresholdPercent = contextPressurePercent(compactThresholdTokens, context.ContextWindow)
		}

		sessions = append(sessions, SessionSnapshot{
			ThreadID:                       row.ThreadID,
			Title:                          row.Title,
			UpdatedAt:                      row.UpdatedAt,
			Model:                          row.Model,
			ReasoningEffort:                row.ReasoningEffort,
			TokensUsedTotal:                row.TokensUsedTotal,
			ContextUsedTokens:              context.ContextUsedTokens,
			ContextWindow:                  context.ContextWindow,
			ContextPressurePercent:         context.ContextPressurePercent,
			ContextCompactThresholdTokens:  compactThresholdTokens,
			ContextCompactThresholdPercent: compactThresholdPercent,
			LastActiveAgoMinutes:           lastActiveAgo,
			ContextCompaction:              contextCompactionForRow(row, compactions[row.ThreadID], now),
		})
	}
	return sessions
}

func (s *Scanner) applyResolvedThreadNames(codexHome string, rows []threadRow) error {
	if len(rows) == 0 || s.resolveThreadNames == nil {
		return nil
	}

	threadIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		threadIDs = append(threadIDs, row.ThreadID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), threadNameResolverTimeout)
	defer cancel()

	resolved, err := s.resolveThreadNames(ctx, codexHome, threadIDs)
	if err != nil {
		return err
	}
	if len(resolved) == 0 {
		return nil
	}
	for i := range rows {
		if name := normalizeThreadName(resolved[rows[i].ThreadID]); name != "" {
			rows[i].Title = name
		}
	}
	return nil
}

func normalizeThreadName(name string) string {
	return strings.TrimSpace(name)
}

func contextPressurePercent(lastUsedTokens, modelContextWindow int64) int {
	effectiveWindow := modelContextWindow - contextPressureBaselineTokens
	if effectiveWindow < 1 {
		effectiveWindow = 1
	}

	effectiveUsed := lastUsedTokens - contextPressureBaselineTokens
	if effectiveUsed < 0 {
		effectiveUsed = 0
	}
	if effectiveUsed > effectiveWindow {
		effectiveUsed = effectiveWindow
	}

	remainingRatio := float64(effectiveWindow-effectiveUsed) / float64(effectiveWindow)
	if remainingRatio < 0 {
		remainingRatio = 0
	}
	if remainingRatio > 1 {
		remainingRatio = 1
	}

	remainingPercent := int(math.Round(remainingRatio * 100))
	pressure := 100 - remainingPercent
	if pressure < 0 {
		return 0
	}
	if pressure > 100 {
		return 100
	}
	return pressure
}

func parseTokenCountEvent(line []byte) (tokenCountEvent, bool) {
	var event tokenCountEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return tokenCountEvent{}, false
	}
	if event.Type != "event_msg" || event.Payload.Type != "token_count" {
		return tokenCountEvent{}, false
	}
	return event, true
}

func loadHeatmapLocation() *time.Location {
	location, err := time.LoadLocation(heatmapTimezoneName)
	if err != nil {
		return time.FixedZone("CST", 8*3600)
	}
	return location
}
