package widgetapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/widget/internal/widgetvm"
)

var ErrCredentialMissing = errors.New("widget credential is not configured")

const DefaultEndpoint = "http://127.0.0.1:8787"

type Client struct {
	endpoint    string
	tokenSource TokenSource
	http        *http.Client
	mu          sync.Mutex
	state       widgetvm.State
	emit        func(widgetvm.State)
	lastEvent   time.Time
}

func NewClient(endpoint string, source TokenSource) *Client {
	if source == nil {
		source = NoTokenSource{}
	}
	return &Client{endpoint: endpoint, tokenSource: source, http: &http.Client{Timeout: 12 * time.Second}, state: widgetvm.InitialState()}
}
func (c *Client) Run(ctx context.Context, emit func(widgetvm.State)) {
	c.mu.Lock()
	c.emit = emit
	c.mu.Unlock()
	c.Refresh(ctx)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkStale()
		}
	}
}
func (c *Client) Refresh(ctx context.Context) {
	c.update(func(s *widgetvm.State) { s.Status = widgetvm.Loading })
	token, err := c.tokenSource.Token()
	if err != nil {
		c.update(func(s *widgetvm.State) { s.Status = widgetvm.Invalid; s.ErrorText = "悬浮球凭据未配置" })
		return
	}
	snapshot, err := c.fetch(ctx, token)
	if err != nil {
		c.update(func(s *widgetvm.State) { s.Status = widgetvm.Offline; s.ErrorText = err.Error() })
		return
	}
	c.update(func(s *widgetvm.State) { *s = snapshot; s.Status = widgetvm.Online })
	go c.stream(ctx, token)
}
func (c *Client) fetch(ctx context.Context, token string) (widgetvm.State, error) {
	u, err := validatedURL(c.endpoint, "/api/status")
	if err != nil {
		return widgetvm.State{}, err
	}
	q := u.Query()
	q.Set("includeDailyTrend30d", "1")
	q.Set("includeSessions", "0")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return widgetvm.State{}, err
	}
	req.Header.Set("X-OpenWatcher-Token", token)
	res, err := c.http.Do(req)
	if err != nil {
		return widgetvm.State{}, fmt.Errorf("服务连接失败")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return widgetvm.State{Status: widgetvm.Invalid}, errors.New("悬浮球凭据已失效")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return widgetvm.State{}, fmt.Errorf("服务暂不可用（HTTP %d）", res.StatusCode)
	}
	var dto response
	if err = json.NewDecoder(res.Body).Decode(&dto); err != nil {
		return widgetvm.State{}, errors.New("状态数据无法解析")
	}
	return dto.state(), nil
}
func (c *Client) stream(ctx context.Context, token string) {
	c.mu.Lock()
	c.lastEvent = time.Now()
	c.mu.Unlock()
	u, err := validatedURL(c.endpoint, "/api/status/stream")
	if err != nil {
		return
	}
	q := u.Query()
	q.Set("includeDailyTrend30d", "1")
	q.Set("includeSessions", "0")
	u.RawQuery = q.Encode()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("X-OpenWatcher-Token", token)
	res, err := c.http.Do(req)
	if err != nil {
		c.update(func(s *widgetvm.State) { s.Status = widgetvm.Reconnecting })
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		c.update(func(s *widgetvm.State) { s.Status = widgetvm.Invalid; s.ErrorText = "悬浮球凭据已失效" })
		return
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		c.update(func(s *widgetvm.State) { s.Status = widgetvm.Reconnecting })
		return
	}
	var event, data string
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		case line == "":
			c.applyEvent(event, data)
			event = ""
			data = ""
		}
	}
	c.update(func(s *widgetvm.State) {
		if s.Status != widgetvm.Invalid {
			s.Status = widgetvm.Reconnecting
		}
	})
}
func (c *Client) applyEvent(kind, data string) {
	c.mu.Lock()
	c.lastEvent = time.Now()
	c.mu.Unlock()
	if kind == "heartbeat" {
		return
	}
	var dto response
	switch kind {
	case "status_snapshot":
		if json.Unmarshal([]byte(data), &dto) == nil {
			c.update(func(s *widgetvm.State) { *s = dto.state(); s.Status = widgetvm.Online })
		}
	case "status_quota":
		var x struct {
			Quota *quotaDTO `json:"quota"`
		}
		if json.Unmarshal([]byte(data), &x) == nil {
			c.update(func(s *widgetvm.State) { s.Quota = x.Quota.vm() })
		}
	case "status_heatmap24h":
		var x response
		if json.Unmarshal([]byte(data), &x) == nil {
			c.update(func(s *widgetvm.State) {
				s.Heatmap24h = x.Heatmap24h.vm()
				s.Heatmap7d = x.Heatmap7d.vm()
				s.Today = x.DailyUsage.today()
			})
		}
	case "status_errors":
		var x struct {
			Errors []string `json:"errors"`
		}
		if json.Unmarshal([]byte(data), &x) == nil {
			c.update(func(s *widgetvm.State) { s.Errors = x.Errors })
		}
	}
}
func (c *Client) checkStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastEvent.IsZero() {
		return
	}
	age := time.Since(c.lastEvent)
	if age > 60*time.Second {
		c.state.Status = widgetvm.Stale
	} else if age > 30*time.Second && c.state.Status == widgetvm.Online {
		c.state.Status = widgetvm.Reconnecting
	} else {
		return
	}
	if c.emit != nil {
		c.emit(c.state)
	}
}
func (c *Client) update(fn func(*widgetvm.State)) {
	c.mu.Lock()
	fn(&c.state)
	s := c.state
	emit := c.emit
	c.mu.Unlock()
	if emit != nil {
		emit(s)
	}
}
func validatedURL(base, path string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(base, "/") + path)
	if err != nil || u.Scheme != "http" && u.Scheme != "https" || u.Hostname() == "" {
		return nil, errors.New("服务地址无效")
	}
	return u, nil
}

type response struct {
	OK            bool          `json:"ok"`
	ObservedAt    string        `json:"observedAt"`
	Quota         *quotaDTO     `json:"quota"`
	Heatmap24h    *heatmap24DTO `json:"heatmap24h"`
	Heatmap7d     *heatmap7DTO  `json:"heatmap7d"`
	DailyUsage    *dailyDTO     `json:"dailyUsage"`
	DailyTrend30d *trendDTO     `json:"dailyTrend30d"`
	Errors        []string      `json:"errors"`
}

func (r response) state() widgetvm.State {
	return widgetvm.State{ObservedAt: r.ObservedAt, Quota: r.Quota.vm(), Heatmap24h: r.Heatmap24h.vm(), Heatmap7d: r.Heatmap7d.vm(), Today: r.DailyUsage.today(), Trend30d: r.DailyTrend30d.vm(), Errors: r.Errors, Timezone: r.Heatmap24h.timezone()}
}

type quotaDTO struct {
	Fresh    bool       `json:"fresh"`
	Status   string     `json:"status"`
	PlanType string     `json:"planType"`
	FiveHour *windowDTO `json:"fiveHour"`
	Weekly   *windowDTO `json:"weekly"`
}
type windowDTO struct {
	RemainingPercent float64 `json:"remainingPercent"`
	ResetAt          int64   `json:"resetAt"`
}

func (q *quotaDTO) vm() *widgetvm.Quota {
	if q == nil {
		return nil
	}
	return &widgetvm.Quota{Fresh: q.Fresh, Status: q.Status, PlanType: q.PlanType, FiveHour: q.FiveHour.vm(), Weekly: q.Weekly.vm()}
}
func (w *windowDTO) vm() *widgetvm.QuotaWindow {
	if w == nil {
		return nil
	}
	return &widgetvm.QuotaWindow{RemainingPercent: w.RemainingPercent, ResetAt: w.ResetAt}
}

type heatmap24DTO struct {
	Timezone string      `json:"timezone"`
	Buckets  []bucketDTO `json:"buckets"`
}
type bucketDTO struct {
	HourStart                                                                        string `json:"hourStart"`
	InputTokens, CachedInputTokens, OutputTokens, ReasoningOutputTokens, TotalTokens int64
	ActiveThreads                                                                    int `json:"activeThreads"`
}

func (h *heatmap24DTO) vm() *widgetvm.Heatmap24h {
	if h == nil {
		return nil
	}
	out := &widgetvm.Heatmap24h{Timezone: h.Timezone}
	for _, b := range h.Buckets {
		out.Buckets = append(out.Buckets, widgetvm.Bucket{HourStart: b.HourStart, InputTokens: b.InputTokens, CachedInputTokens: b.CachedInputTokens, OutputTokens: b.OutputTokens, ReasoningOutputTokens: b.ReasoningOutputTokens, TotalTokens: b.TotalTokens, ActiveThreads: b.ActiveThreads})
	}
	return out
}
func (h *heatmap24DTO) timezone() string {
	if h == nil {
		return ""
	}
	return h.Timezone
}

type heatmap7DTO struct {
	Timezone, StartDate, EndDate string
	PeakTokens                   int64
	Days                         []heatmapDayDTO
}
type heatmapDayDTO struct {
	Date        string
	TotalTokens int64
	Hours       []int64
}

func (h *heatmap7DTO) vm() *widgetvm.Heatmap7d {
	if h == nil {
		return nil
	}
	o := &widgetvm.Heatmap7d{Timezone: h.Timezone, StartDate: h.StartDate, EndDate: h.EndDate, PeakTokens: h.PeakTokens}
	for _, d := range h.Days {
		o.Days = append(o.Days, widgetvm.HeatmapDay{Date: d.Date, TotalTokens: d.TotalTokens, Hours: d.Hours})
	}
	return o
}

type dailyDTO struct {
	InputTokens, CachedInputTokens, OutputTokens, ReasoningOutputTokens, TotalTokens int64
	EstimatedValueLabel                                                              string `json:"estimatedValueLabel"`
}

func (d *dailyDTO) today() *widgetvm.Today {
	if d == nil {
		return nil
	}
	return &widgetvm.Today{InputTokens: d.InputTokens, CachedInputTokens: d.CachedInputTokens, OutputTokens: d.OutputTokens, ReasoningOutputTokens: d.ReasoningOutputTokens, TotalTokens: d.TotalTokens, ValueLabel: d.EstimatedValueLabel}
}

type trendDTO struct {
	Timezone, StartDate, EndDate           string
	TotalTokens, AverageTokens, PeakTokens int64
	EstimatedValueLabel                    string `json:"estimatedValueLabel"`
	Days                                   []trendDayDTO
}
type trendDayDTO struct {
	Date        string
	TotalTokens int64
}

func (t *trendDTO) vm() *widgetvm.Trend30d {
	if t == nil {
		return nil
	}
	o := &widgetvm.Trend30d{Timezone: t.Timezone, StartDate: t.StartDate, EndDate: t.EndDate, TotalTokens: t.TotalTokens, AverageTokens: t.AverageTokens, PeakTokens: t.PeakTokens, ValueLabel: t.EstimatedValueLabel}
	for _, d := range t.Days {
		o.Days = append(o.Days, widgetvm.TrendDay{Date: d.Date, TotalTokens: d.TotalTokens})
	}
	return o
}
