package server

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"openwatcher/internal/sessions"
)

const openAIPricingSourceURL = "https://developers.openai.com/api/docs/pricing"
const openAIPricingFetchTimeout = 5 * time.Second

type dailyPricingCache struct {
	mu        sync.Mutex
	client    *http.Client
	sourceURL string
}

type modelPricingSnapshot struct {
	Date      string                  `json:"date"`
	SourceURL string                  `json:"sourceUrl"`
	Source    string                  `json:"source,omitempty"`
	FetchedAt string                  `json:"fetchedAt"`
	Models    map[string]modelPricing `json:"models"`
}

type modelPricing struct {
	InputPerMillion       float64 `json:"inputPerMillion"`
	CachedInputPerMillion float64 `json:"cachedInputPerMillion"`
	OutputPerMillion      float64 `json:"outputPerMillion"`
}

type dailyUsageResponse struct {
	GeneratedAt              string                 `json:"generatedAt,omitempty"`
	TotalTokens              int64                  `json:"totalTokens"`
	InputTokens              int64                  `json:"inputTokens"`
	CachedInputTokens        int64                  `json:"cachedInputTokens"`
	OutputTokens             int64                  `json:"outputTokens"`
	ReasoningOutputTokens    int64                  `json:"reasoningOutputTokens"`
	ActiveSessions           int                    `json:"activeSessions"`
	EstimatedValueUSD        *float64               `json:"estimatedValueUsd,omitempty"`
	EstimatedValueLabel      string                 `json:"estimatedValueLabel,omitempty"`
	PricingDate              string                 `json:"pricingDate,omitempty"`
	PricingSourceURL         string                 `json:"pricingSourceUrl,omitempty"`
	PricingSource            string                 `json:"pricingSource,omitempty"`
	PricingUnavailableReason string                 `json:"pricingUnavailableReason,omitempty"`
	ModelShares              []dailyUsageModelShare `json:"modelShares,omitempty"`
}

type dailyUsageModelShare struct {
	Model        string  `json:"model"`
	Tokens       int64   `json:"tokens"`
	SharePercent float64 `json:"sharePercent"`
}

type dailyTrend30dResponse struct {
	Timezone                 string                   `json:"timezone"`
	GeneratedAt              string                   `json:"generatedAt,omitempty"`
	StartDate                string                   `json:"startDate"`
	EndDate                  string                   `json:"endDate"`
	TotalTokens              int64                    `json:"totalTokens"`
	AverageTokens            int64                    `json:"averageTokens"`
	PeakTokens               int64                    `json:"peakTokens"`
	Days                     []sessions.DailyTrendDay `json:"days"`
	EstimatedValueUSD        *float64                 `json:"estimatedValueUsd,omitempty"`
	EstimatedValueLabel      string                   `json:"estimatedValueLabel,omitempty"`
	PricingDate              string                   `json:"pricingDate,omitempty"`
	PricingSourceURL         string                   `json:"pricingSourceUrl,omitempty"`
	PricingSource            string                   `json:"pricingSource,omitempty"`
	PricingUnavailableReason string                   `json:"pricingUnavailableReason,omitempty"`
}

func newDailyPricingCache() *dailyPricingCache {
	return &dailyPricingCache{
		client:    &http.Client{Timeout: openAIPricingFetchTimeout},
		sourceURL: openAIPricingSourceURL,
	}
}

func (c *dailyPricingCache) snapshotForDay(now time.Time) (modelPricingSnapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, err := dailyPricingCachePath(now)
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	if snapshot, err := readModelPricingSnapshot(path); err == nil && len(snapshot.Models) > 0 {
		return supplementModelPricingSnapshot(snapshot, now), nil
	}

	snapshot, fetchErr := c.fetchSnapshotForDay(now)
	if fetchErr != nil {
		snapshot = defaultModelPricingSnapshot(now)
		snapshot.Source = "built-in-fallback"
	}
	snapshot = supplementModelPricingSnapshot(snapshot, now)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return modelPricingSnapshot{}, err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	data = append(data, '\n')
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return modelPricingSnapshot{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return modelPricingSnapshot{}, err
	}
	_ = os.Chmod(path, 0o600)
	return snapshot, nil
}

func (c *dailyPricingCache) fetchSnapshotForDay(now time.Time) (modelPricingSnapshot, error) {
	req, err := http.NewRequest(http.MethodGet, c.sourceURL, nil)
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	req.Header.Set("User-Agent", "openwatcher/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return modelPricingSnapshot{}, fmt.Errorf("pricing http status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	models, err := parseOpenAITextModelPricingHTML(string(body))
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	return modelPricingSnapshot{
		Date:      now.Format("2006-01-02"),
		SourceURL: c.sourceURL,
		Source:    "openai-docs",
		FetchedAt: now.Format(time.RFC3339),
		Models:    models,
	}, nil
}

func dailyPricingCachePath(now time.Time) (string, error) {
	if cacheDir := strings.TrimSpace(os.Getenv("OPENWATCHER_CACHE_DIR")); cacheDir != "" {
		day := now.Format("2006-01-02")
		return filepath.Join(cacheDir, "openai-pricing-"+day+".json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	day := now.Format("2006-01-02")
	return filepath.Join(home, ".openwatcher", "cache", "openai-pricing-"+day+".json"), nil
}

func readModelPricingSnapshot(path string) (modelPricingSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return modelPricingSnapshot{}, err
	}
	var snapshot modelPricingSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return modelPricingSnapshot{}, err
	}
	return snapshot, nil
}

func defaultModelPricingSnapshot(now time.Time) modelPricingSnapshot {
	return modelPricingSnapshot{
		Date:      now.Format("2006-01-02"),
		SourceURL: openAIPricingSourceURL,
		Source:    "built-in-fallback",
		FetchedAt: now.Format(time.RFC3339),
		Models: map[string]modelPricing{
			"gpt-5.6-sol":       {InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00},
			"gpt-5.6-terra":     {InputPerMillion: 2.50, CachedInputPerMillion: 0.25, OutputPerMillion: 15.00},
			"gpt-5.6-luna":      {InputPerMillion: 1.00, CachedInputPerMillion: 0.10, OutputPerMillion: 6.00},
			"gpt-5.5":           {InputPerMillion: 5.00, CachedInputPerMillion: 0.50, OutputPerMillion: 30.00},
			"gpt-5.5-pro":       {InputPerMillion: 30.00, CachedInputPerMillion: 0.00, OutputPerMillion: 180.00},
			"gpt-5.4":           {InputPerMillion: 2.50, CachedInputPerMillion: 0.25, OutputPerMillion: 15.00},
			"gpt-5.4-mini":      {InputPerMillion: 0.75, CachedInputPerMillion: 0.075, OutputPerMillion: 4.50},
			"gpt-5.4-nano":      {InputPerMillion: 0.20, CachedInputPerMillion: 0.02, OutputPerMillion: 1.25},
			"gpt-5.4-pro":       {InputPerMillion: 30.00, CachedInputPerMillion: 0.00, OutputPerMillion: 180.00},
			"gpt-5.3-codex":     {InputPerMillion: 1.75, CachedInputPerMillion: 0.175, OutputPerMillion: 14.00},
			"gpt-5.2":           {InputPerMillion: 1.75, CachedInputPerMillion: 0.175, OutputPerMillion: 14.00},
			"gpt-5.2-codex":     {InputPerMillion: 1.75, CachedInputPerMillion: 0.175, OutputPerMillion: 14.00},
			"gpt-5.2-pro":       {InputPerMillion: 21.00, CachedInputPerMillion: 0.00, OutputPerMillion: 168.00},
			"gpt-5.1":           {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
			"gpt-5.1-codex-max": {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
			"gpt-5.1-codex":     {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
			"gpt-5":             {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
			"gpt-5-codex":       {InputPerMillion: 1.25, CachedInputPerMillion: 0.125, OutputPerMillion: 10.00},
			"gpt-5-mini":        {InputPerMillion: 0.25, CachedInputPerMillion: 0.025, OutputPerMillion: 2.00},
			"gpt-5-nano":        {InputPerMillion: 0.05, CachedInputPerMillion: 0.005, OutputPerMillion: 0.40},
			"gpt-5-pro":         {InputPerMillion: 15.00, CachedInputPerMillion: 0.00, OutputPerMillion: 120.00},
			"gpt-4.1":           {InputPerMillion: 2.00, CachedInputPerMillion: 0.50, OutputPerMillion: 8.00},
			"gpt-4.1-mini":      {InputPerMillion: 0.40, CachedInputPerMillion: 0.10, OutputPerMillion: 1.60},
			"gpt-4.1-nano":      {InputPerMillion: 0.10, CachedInputPerMillion: 0.025, OutputPerMillion: 0.40},
			"gpt-4o":            {InputPerMillion: 2.50, CachedInputPerMillion: 1.25, OutputPerMillion: 10.00},
			"gpt-4o-mini":       {InputPerMillion: 0.15, CachedInputPerMillion: 0.075, OutputPerMillion: 0.60},
		},
	}
}

func supplementModelPricingSnapshot(snapshot modelPricingSnapshot, now time.Time) modelPricingSnapshot {
	fallback := defaultModelPricingSnapshot(now)
	if snapshot.Models == nil {
		snapshot.Models = make(map[string]modelPricing, len(fallback.Models))
	}
	added := false
	for model, pricing := range fallback.Models {
		if _, exists := snapshot.Models[model]; exists {
			continue
		}
		snapshot.Models[model] = pricing
		added = true
	}
	if added && snapshot.Source != "built-in-fallback" && !strings.Contains(snapshot.Source, "+built-in-fallback") {
		if snapshot.Source == "" {
			snapshot.Source = "built-in-fallback"
		} else {
			snapshot.Source += "+built-in-fallback"
		}
	}
	return snapshot
}

func parseOpenAITextModelPricingHTML(content string) (map[string]modelPricing, error) {
	decoded := html.UnescapeString(content)
	rowPattern := regexp.MustCompile(`\[1,\[\[0,"([^"]+)"\],\[0,([^\]]+)\],\[0,([^\]]+)\],\[0,([^\]]+)\]\]\]`)
	matches := rowPattern.FindAllStringSubmatch(decoded, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("pricing rows not found")
	}

	models := map[string]modelPricing{}
	for _, match := range matches {
		model := normalizePricingModelName(match[1])
		if model == "" {
			continue
		}
		if _, exists := models[model]; exists {
			continue
		}
		input, ok := parsePricingNumber(match[2])
		if !ok {
			continue
		}
		cached, _ := parsePricingNumber(match[3])
		output, ok := parsePricingNumber(match[4])
		if !ok {
			continue
		}
		models[model] = modelPricing{
			InputPerMillion:       input,
			CachedInputPerMillion: cached,
			OutputPerMillion:      output,
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("usable pricing rows not found")
	}
	return models, nil
}

func parsePricingNumber(value string) (float64, bool) {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.Trim(cleaned, `"`)
	if cleaned == "" || cleaned == "-" || cleaned == "null" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func normalizePricingModelName(model string) string {
	value := strings.ToLower(strings.TrimSpace(model))
	if paren := strings.Index(value, " ("); paren >= 0 {
		value = strings.TrimSpace(value[:paren])
	}
	return value
}

func buildDailyUsageResponse(usage sessions.DailyTokenUsage, pricing modelPricingSnapshot, pricingErr error) dailyUsageResponse {
	response := dailyUsageResponse{
		GeneratedAt:           usage.GeneratedAt.Format(time.RFC3339),
		TotalTokens:           usage.TotalTokens,
		InputTokens:           usage.InputTokens,
		CachedInputTokens:     usage.CachedInputTokens,
		OutputTokens:          usage.OutputTokens,
		ReasoningOutputTokens: usage.ReasoningOutputTokens,
		ActiveSessions:        usage.ActiveSessions,
		ModelShares:           buildModelShares(usage.ModelTokenBreakdowns, usage.TotalTokens),
	}
	if pricingErr != nil {
		response.PricingUnavailableReason = "pricing unavailable"
		return response
	}
	response.PricingDate = pricing.Date
	response.PricingSourceURL = pricing.SourceURL
	response.PricingSource = pricing.Source

	totalValue, ok := estimateDailyValueUSD(usage.ModelTokenBreakdowns, pricing.Models)
	if !ok {
		response.PricingUnavailableReason = "model pricing unavailable"
		return response
	}
	totalValue = math.Round(totalValue*10000) / 10000
	response.EstimatedValueUSD = &totalValue
	response.EstimatedValueLabel = formatUSDValue(totalValue)
	return response
}

func buildDailyTrend30dResponse(trend sessions.DailyTrend30dSnapshot, pricing modelPricingSnapshot, pricingErr error) dailyTrend30dResponse {
	response := dailyTrend30dResponse{
		Timezone:      trend.Timezone,
		GeneratedAt:   trend.GeneratedAt.Format(time.RFC3339),
		StartDate:     trend.StartDate,
		EndDate:       trend.EndDate,
		TotalTokens:   trend.TotalTokens,
		AverageTokens: trend.AverageTokens,
		PeakTokens:    trend.PeakTokens,
		Days:          append([]sessions.DailyTrendDay(nil), trend.Days...),
	}
	if pricingErr != nil {
		response.PricingUnavailableReason = "pricing unavailable"
		return response
	}
	response.PricingDate = pricing.Date
	response.PricingSourceURL = pricing.SourceURL
	response.PricingSource = pricing.Source
	totalValue, ok := estimateDailyValueUSD(trend.ModelTokenBreakdowns, pricing.Models)
	if !ok {
		response.PricingUnavailableReason = "model pricing unavailable"
		return response
	}
	totalValue = math.Round(totalValue*100) / 100
	response.EstimatedValueUSD = &totalValue
	response.EstimatedValueLabel = formatUSDValue(totalValue)
	return response
}

func buildModelShares(models []sessions.DailyModelTokenUsage, totalTokens int64) []dailyUsageModelShare {
	if totalTokens <= 0 {
		return nil
	}
	shares := make([]dailyUsageModelShare, 0, len(models))
	for _, model := range models {
		if model.TotalTokens <= 0 {
			continue
		}
		shares = append(shares, dailyUsageModelShare{
			Model:        model.Model,
			Tokens:       model.TotalTokens,
			SharePercent: math.Round((float64(model.TotalTokens)*1000)/float64(totalTokens)) / 10,
		})
	}
	sort.Slice(shares, func(i, j int) bool {
		return shares[i].Tokens > shares[j].Tokens
	})
	if len(shares) > 4 {
		return shares[:4]
	}
	return shares
}

func estimateDailyValueUSD(models []sessions.DailyModelTokenUsage, prices map[string]modelPricing) (float64, bool) {
	var total float64
	usedPricedModel := false
	for _, usage := range models {
		price, ok := prices[normalizeModelForPricing(usage.Model)]
		if !ok {
			continue
		}
		usedPricedModel = true
		uncachedInput := maxInt64(0, usage.InputTokens-usage.CachedInputTokens)
		// Codex token_count reports reasoning_output_tokens as a detail inside output_tokens.
		output := usage.OutputTokens
		total += float64(uncachedInput) * price.InputPerMillion / 1_000_000
		total += float64(usage.CachedInputTokens) * price.CachedInputPerMillion / 1_000_000
		total += float64(output) * price.OutputPerMillion / 1_000_000
	}
	return total, usedPricedModel
}

func normalizeModelForPricing(model string) string {
	value := strings.ToLower(strings.TrimSpace(model))
	if strings.HasSuffix(value, "-latest") {
		value = strings.TrimSuffix(value, "-latest")
	}
	return value
}

func formatUSDValue(value float64) string {
	switch {
	case value <= 0:
		return "$0.00"
	case value < 0.01:
		return fmt.Sprintf("$%.4f", value)
	case value < 1:
		return fmt.Sprintf("$%.2f", value)
	default:
		return fmt.Sprintf("$%.2f", value)
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
