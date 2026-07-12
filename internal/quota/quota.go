package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"openwatcher/internal/auth"
)

const DefaultUsageEndpoint = "https://chatgpt.com/backend-api/wham/usage"

const (
	fiveHourWindowSeconds = int64((5 * time.Hour) / time.Second)
	weeklyWindowSeconds   = int64((7 * 24 * time.Hour) / time.Second)
)

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	CodexHome string
	Endpoint  string
	HTTP      HTTPClient
}

type Snapshot struct {
	Source   string  `json:"source"`
	Fresh    bool    `json:"fresh"`
	Status   Status  `json:"status"`
	PlanType string  `json:"planType,omitempty"`
	FiveHour *Window `json:"fiveHour,omitempty"`
	Weekly   *Window `json:"weekly,omitempty"`
}

type Status string

const (
	StatusOK          Status = "ok"
	StatusStale       Status = "stale"
	StatusUnavailable Status = "unavailable"
)

type Window struct {
	UsedPercent      float64 `json:"usedPercent"`
	RemainingPercent float64 `json:"remainingPercent"`
	ResetAt          int64   `json:"resetAt"`
}

func NewClient(codexHome string) *Client {
	return &Client{
		CodexHome: codexHome,
		Endpoint:  DefaultUsageEndpoint,
		HTTP:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Fetch(ctx context.Context) (*Snapshot, error) {
	creds, err := auth.ReadCredentials(c.CodexHome)
	if err != nil {
		return nil, fmt.Errorf("codex auth unavailable: %w", err)
	}

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = DefaultUsageEndpoint
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "openwatcher/1.0")
	req.Header.Set("Origin", "https://chatgpt.com")
	req.Header.Set("Referer", "https://chatgpt.com/")
	if creds.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", creds.AccountID)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("usage api request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("usage api returned HTTP %d", res.StatusCode)
	}

	decoder := json.NewDecoder(res.Body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	return ParseUsage(raw, time.Now())
}

func ParseUsage(data []byte, now time.Time) (*Snapshot, error) {
	var payload usageResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	primary, err := normalizeWindow(payload.RateLimit.PrimaryWindow, now)
	if err != nil {
		return nil, fmt.Errorf("primary window: %w", err)
	}
	secondary, err := normalizeWindow(payload.RateLimit.SecondaryWindow, now)
	if err != nil {
		return nil, fmt.Errorf("secondary window: %w", err)
	}
	fiveHour, weekly := classifyWindows(
		payload.RateLimit.PrimaryWindow,
		primary,
		payload.RateLimit.SecondaryWindow,
		secondary,
	)

	return &Snapshot{
		Source:   "oauth-api",
		Fresh:    true,
		Status:   StatusOK,
		PlanType: payload.PlanType,
		FiveHour: fiveHour,
		Weekly:   weekly,
	}, nil
}

type usageResponse struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow   *usageWindow `json:"primary_window"`
		SecondaryWindow *usageWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

type usageWindow struct {
	UsedPercent        flexibleFloat `json:"used_percent"`
	LimitWindowSeconds flexibleInt   `json:"limit_window_seconds"`
	ResetAt            flexibleInt   `json:"reset_at"`
	ResetsAt           flexibleInt   `json:"resets_at"`
	ResetAfterSeconds  flexibleInt   `json:"reset_after_seconds"`
}

type windowKind uint8

const (
	windowKindLegacy windowKind = iota
	windowKindFiveHour
	windowKindWeekly
	windowKindUnsupported
)

func classifyWindows(
	primaryInput *usageWindow,
	primary *Window,
	secondaryInput *usageWindow,
	secondary *Window,
) (*Window, *Window) {
	var fiveHour, weekly *Window

	assignKnownWindow := func(input *usageWindow, window *Window) {
		switch classifyWindow(input) {
		case windowKindFiveHour:
			if fiveHour == nil {
				fiveHour = window
			}
		case windowKindWeekly:
			if weekly == nil {
				weekly = window
			}
		}
	}
	assignKnownWindow(primaryInput, primary)
	assignKnownWindow(secondaryInput, secondary)

	// 旧响应未提供窗口周期，仅在缺少明确周期时兼容 primary=5h、secondary=7d；
	// 已提供但不匹配的周期不能被错误标记为现有窗口。
	if primary != nil && classifyWindow(primaryInput) == windowKindLegacy && fiveHour == nil {
		fiveHour = primary
	}
	if secondary != nil && classifyWindow(secondaryInput) == windowKindLegacy && weekly == nil {
		weekly = secondary
	}

	return fiveHour, weekly
}

func classifyWindow(input *usageWindow) windowKind {
	if input == nil || !input.LimitWindowSeconds.Set {
		return windowKindLegacy
	}
	switch input.LimitWindowSeconds.Value {
	case fiveHourWindowSeconds:
		return windowKindFiveHour
	case weeklyWindowSeconds:
		return windowKindWeekly
	default:
		return windowKindUnsupported
	}
}

type flexibleFloat struct {
	Value float64
	Set   bool
}

func (n *flexibleFloat) UnmarshalJSON(data []byte) error {
	value, ok, err := parseNumber(data)
	if err != nil {
		return err
	}
	n.Value = value
	n.Set = ok
	return nil
}

type flexibleInt struct {
	Value int64
	Set   bool
}

func (n *flexibleInt) UnmarshalJSON(data []byte) error {
	value, ok, err := parseNumber(data)
	if err != nil {
		return err
	}
	n.Value = int64(math.Round(value))
	n.Set = ok
	return nil
}

func parseNumber(data []byte) (float64, bool, error) {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return 0, false, nil
	}
	if strings.HasPrefix(text, "\"") {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return 0, false, err
		}
		if s == "" {
			return 0, false, nil
		}
		v, err := strconv.ParseFloat(s, 64)
		return v, true, err
	}
	v, err := strconv.ParseFloat(text, 64)
	return v, true, err
}

func normalizeWindow(input *usageWindow, now time.Time) (*Window, error) {
	if input == nil {
		return nil, nil
	}
	if !input.UsedPercent.Set {
		return nil, errors.New("used_percent missing")
	}

	resetAt := int64(0)
	switch {
	case input.ResetAt.Set:
		resetAt = input.ResetAt.Value
	case input.ResetsAt.Set:
		resetAt = input.ResetsAt.Value
	case input.ResetAfterSeconds.Set:
		resetAt = now.Unix() + input.ResetAfterSeconds.Value
	}

	used := round2(input.UsedPercent.Value)
	remaining := round2(100 - used)
	if remaining < 0 {
		remaining = 0
	}
	if remaining > 100 {
		remaining = 100
	}

	return &Window{
		UsedPercent:      used,
		RemainingPercent: remaining,
		ResetAt:          resetAt,
	}, nil
}

type Refresher struct {
	client   *Client
	interval time.Duration

	mu       sync.RWMutex
	snapshot *Snapshot
	status   Status
}

func NewRefresher(client *Client, interval time.Duration) *Refresher {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Refresher{client: client, interval: interval, status: StatusUnavailable}
}

func (r *Refresher) Start(ctx context.Context) {
	go func() {
		r.Refresh(ctx)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.Refresh(ctx)
			}
		}
	}()
}

func (r *Refresher) Refresh(ctx context.Context) {
	snapshot, err := r.client.Fetch(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		if r.snapshot != nil {
			r.snapshot.Fresh = false
			r.snapshot.Status = StatusStale
			r.status = StatusStale
		} else {
			r.status = StatusUnavailable
		}
		return
	}
	r.snapshot = cloneSnapshot(snapshot)
	r.snapshot.Fresh = true
	r.snapshot.Status = StatusOK
	r.status = StatusOK
}

func (r *Refresher) Snapshot() (*Snapshot, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.snapshot == nil {
		return &Snapshot{Source: "oauth-api", Fresh: false, Status: StatusUnavailable}, nil
	}
	output := cloneSnapshot(r.snapshot)
	switch {
	case output.Status != "":
	case output.Fresh:
		output.Status = StatusOK
	default:
		output.Status = StatusStale
	}
	return output, nil
}

func cloneSnapshot(input *Snapshot) *Snapshot {
	if input == nil {
		return nil
	}
	output := *input
	if input.FiveHour != nil {
		fiveHour := *input.FiveHour
		output.FiveHour = &fiveHour
	}
	if input.Weekly != nil {
		weekly := *input.Weekly
		output.Weekly = &weekly
	}
	return &output
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
