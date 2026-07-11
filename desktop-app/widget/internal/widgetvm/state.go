package widgetvm

import "time"

type ConnectionStatus string

const (
	Loading      ConnectionStatus = "loading"
	Online       ConnectionStatus = "online"
	Reconnecting ConnectionStatus = "reconnecting"
	Stale        ConnectionStatus = "stale"
	Offline      ConnectionStatus = "offline"
	Invalid      ConnectionStatus = "invalid_credential"
	Partial      ConnectionStatus = "partial"
)

type State struct {
	Status       ConnectionStatus `json:"status"`
	Expanded     bool             `json:"expanded"`
	ObservedAt   string           `json:"observedAt,omitempty"`
	Timezone     string           `json:"timezone,omitempty"`
	LastActionAt time.Time        `json:"-"`
	Quota        *Quota           `json:"quota,omitempty"`
	Heatmap24h   *Heatmap24h      `json:"heatmap24h,omitempty"`
	Heatmap7d    *Heatmap7d       `json:"heatmap7d,omitempty"`
	Today        *Today           `json:"today,omitempty"`
	Trend30d     *Trend30d        `json:"trend30d,omitempty"`
	Errors       []string         `json:"errors,omitempty"`
	ErrorText    string           `json:"errorText,omitempty"`
}

type Quota struct {
	PlanType string       `json:"planType,omitempty"`
	Fresh    bool         `json:"fresh"`
	Status   string       `json:"status"`
	FiveHour *QuotaWindow `json:"fiveHour,omitempty"`
	Weekly   *QuotaWindow `json:"weekly,omitempty"`
}
type QuotaWindow struct {
	RemainingPercent float64 `json:"remainingPercent"`
	ResetAt          int64   `json:"resetAt"`
}
type Heatmap24h struct {
	Timezone string   `json:"timezone"`
	Buckets  []Bucket `json:"buckets"`
}
type Bucket struct {
	HourStart             string `json:"hourStart"`
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	ActiveThreads         int    `json:"activeThreads"`
}
type Heatmap7d struct {
	Timezone   string       `json:"timezone"`
	StartDate  string       `json:"startDate"`
	EndDate    string       `json:"endDate"`
	PeakTokens int64        `json:"peakTokens"`
	Days       []HeatmapDay `json:"days"`
}
type HeatmapDay struct {
	Date        string  `json:"date"`
	TotalTokens int64   `json:"totalTokens"`
	Hours       []int64 `json:"hours"`
}
type Today struct {
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	ValueLabel            string `json:"valueLabel,omitempty"`
}
type Trend30d struct {
	Timezone      string     `json:"timezone"`
	StartDate     string     `json:"startDate"`
	EndDate       string     `json:"endDate"`
	TotalTokens   int64      `json:"totalTokens"`
	AverageTokens int64      `json:"averageTokens"`
	PeakTokens    int64      `json:"peakTokens"`
	ValueLabel    string     `json:"valueLabel,omitempty"`
	Days          []TrendDay `json:"days"`
}
type TrendDay struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"totalTokens"`
}

func InitialState() State { return State{Status: Loading} }
