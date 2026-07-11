package widgetapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/widgettransport"
	"openwatcher/desktop-app/widget/internal/widgetvm"
)

var ErrCredentialMissing = errors.New("widget credential is not configured")

const maxBody = 2 << 20

type Clock interface {
	Now() time.Time
	After(time.Duration) <-chan time.Time
}

type TrendStore interface {
	LoadTrend30d() *widgetvm.Trend30d
	SaveTrend30d(*widgetvm.Trend30d) error
}

type NoTrendStore struct{}

func (NoTrendStore) LoadTrend30d() *widgetvm.Trend30d      { return nil }
func (NoTrendStore) SaveTrend30d(*widgetvm.Trend30d) error { return nil }

type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

type Client struct {
	endpoint                 string
	tokenSource              TokenSource
	trendStore               TrendStore
	snapshotHTTP, streamHTTP *http.Client
	clock                    Clock
	mu                       sync.Mutex
	state                    widgetvm.State
	emit                     func(widgetvm.State)
	refresh                  chan struct{}
	activeCancel             context.CancelFunc
	lastActivity             time.Time
}

func NewClient(endpoint string, source TokenSource, trends ...TrendStore) *Client {
	return NewClientWithClock(endpoint, source, realClock{}, trends...)
}
func NewClientWithClock(endpoint string, source TokenSource, clock Clock, trends ...TrendStore) *Client {
	if source == nil {
		source = NoTokenSource{}
	}
	if clock == nil {
		clock = realClock{}
	}
	trendStore := TrendStore(NoTrendStore{})
	if len(trends) > 0 && trends[0] != nil {
		trendStore = trends[0]
	}
	state := widgetvm.InitialState()
	if cached := trendStore.LoadTrend30d(); trendIsCurrent(cached, clock.Now()) {
		state.Trend30d = cached
	}
	return &Client{
		endpoint:    endpoint,
		tokenSource: source,
		trendStore:  trendStore,
		snapshotHTTP: &http.Client{
			Timeout:   12 * time.Second,
			Transport: loopbackTransport(),
		},
		streamHTTP: &http.Client{Transport: loopbackTransport()},
		clock:      clock,
		state:      state,
		refresh:    make(chan struct{}, 1),
	}
}

func loopbackTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      false,
		MaxIdleConns:           2,
		MaxIdleConnsPerHost:    2,
		IdleConnTimeout:        30 * time.Second,
		ResponseHeaderTimeout:  12 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
	}
}
func ValidEndpoint(endpoint string) bool {
	_, err := widgettransport.ParseEndpoint(endpoint)
	return err == nil
}

// Run owns the sole HTTP/SSE loop. Every reconnect begins with a full GET.
func (c *Client) Run(ctx context.Context, emit func(widgetvm.State)) {
	c.mu.Lock()
	c.emit = emit
	initial := c.state
	c.mu.Unlock()
	if initial.Trend30d != nil && emit != nil {
		emit(initial)
	}
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		token, err := c.tokenSource.Token()
		if err != nil {
			c.invalid("悬浮球凭据未配置")
			if !c.retryWait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		snapshot, code, err := c.fetch(ctx, token)
		if err != nil {
			if code == http.StatusUnauthorized {
				c.invalid("悬浮球凭据已失效")
			} else {
				c.connectionUnavailable()
			}
			if !c.retryWait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		c.update(func(s *widgetvm.State) {
			expanded := s.Expanded
			anchorCorner := s.AnchorCorner
			trend := s.Trend30d
			*s = snapshot
			if s.Trend30d == nil {
				s.Trend30d = trend
			}
			s.Expanded = expanded
			s.AnchorCorner = anchorCorner
			s.Status = widgetvm.Online
			s.ErrorText = ""
		})
		c.markActivity()
		backoff = time.Second
		manual, auth := c.stream(ctx, token)
		if auth {
			c.invalid("悬浮球凭据已失效")
			if !c.retryWait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}
		if manual {
			backoff = time.Second
			continue
		}
		c.connectionUnavailable()
		if !c.retryWait(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

// Refresh cancels the active stream and asks the owner loop for a complete snapshot.
func (c *Client) Refresh() {
	c.mu.Lock()
	if c.activeCancel != nil {
		c.activeCancel()
	}
	c.mu.Unlock()
	select {
	case c.refresh <- struct{}{}:
	default:
	}
}
func (c *Client) wait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-c.refresh:
		return true
	case <-c.clock.After(d):
		return true
	}
}
func (c *Client) retryWait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return c.wait(ctx, 0)
	}
	// A small random spread prevents multiple helpers from reconnecting in the
	// same instant after a service restart while preserving the documented cap.
	return c.wait(ctx, retryDelay(d))
}
func retryDelay(d time.Duration) time.Duration {
	factor := 0.85 + rand.Float64()*0.15
	return time.Duration(float64(d) * factor)
}
func nextBackoff(d time.Duration) time.Duration {
	switch d {
	case time.Second:
		return 2 * time.Second
	case 2 * time.Second:
		return 5 * time.Second
	case 5 * time.Second:
		return 10 * time.Second
	default:
		return 30 * time.Second
	}
}
func (c *Client) fetch(ctx context.Context, token string) (widgetvm.State, int, error) {
	u, err := validatedURL(c.endpoint, "/api/status")
	if err != nil {
		return widgetvm.State{}, 0, err
	}
	req, err := request(ctx, u, token, false, false)
	if err != nil {
		return widgetvm.State{}, 0, err
	}
	res, err := c.snapshotHTTP.Do(req)
	if err != nil {
		return widgetvm.State{}, 0, errors.New("服务连接失败")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return widgetvm.State{}, res.StatusCode, errors.New("服务暂不可用")
	}
	var dto response
	if err := json.NewDecoder(io.LimitReader(res.Body, maxBody)).Decode(&dto); err != nil {
		return widgetvm.State{}, res.StatusCode, errors.New("状态数据无法解析")
	}
	return dto.state(), res.StatusCode, nil
}
func request(ctx context.Context, u *url.URL, token string, stream, includeDailyTrend30d bool) (*http.Request, error) {
	q := u.Query()
	if includeDailyTrend30d {
		q.Set("includeDailyTrend30d", "1")
	} else {
		q.Del("includeDailyTrend30d")
	}
	q.Set("includeSessions", "0")
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err == nil {
		req.Header.Set("X-OpenWatcher-Token", token)
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}
	}
	return req, err
}

// stream has no Client.Timeout. Freshness is driven by received SSE frames, including heartbeat.
func (c *Client) stream(parent context.Context, token string) (manual, auth bool) {
	ctx, cancel := context.WithCancel(parent)
	c.mu.Lock()
	c.activeCancel = cancel
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		if c.activeCancel != nil {
			c.activeCancel = nil
		}
		c.mu.Unlock()
	}()
	u, err := validatedURL(c.endpoint, "/api/status/stream")
	if err != nil {
		return false, false
	}
	req, err := request(ctx, u, token, true, c.needsDailyTrend30d())
	if err != nil {
		return false, false
	}
	res, err := c.streamHTTP.Do(req)
	if err != nil {
		return c.wasManual(), false
	}
	if res.StatusCode == http.StatusUnauthorized {
		res.Body.Close()
		return false, true
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		res.Body.Close()
		return c.wasManual(), false
	}
	defer res.Body.Close()
	events := make(chan sseEvent, 8)
	done := make(chan error, 1)
	go func() { done <- scanSSE(ctx, res.Body, events) }()
	tick := c.clock.After(time.Second)
	for {
		select {
		case <-parent.Done():
			return false, false
		case <-c.refresh:
			return true, false
		case ev, ok := <-events:
			if !ok {
				return c.wasManual(), false
			}
			c.applyEvent(ev.kind, ev.data)
			tick = c.clock.After(time.Second)
		case <-done:
			return c.wasManual(), false
		case <-tick:
			age := c.age()
			if age >= 60*time.Second {
				c.setStatus(widgetvm.Stale, "数据可能已过期")
				cancel()
				return false, false
			}
			if age >= 30*time.Second {
				c.setStatus(widgetvm.Reconnecting, "正在重连")
				cancel()
				return false, false
			}
			tick = c.clock.After(time.Second)
		}
	}
}

type sseEvent struct{ kind, data string }

func scanSSE(ctx context.Context, r io.Reader, out chan<- sseEvent) error {
	defer close(out)
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 4096), maxBody)
	var kind string
	var data []string
	var eventBytes int
	send := func() error {
		event := sseEvent{kind, strings.Join(data, "\n")}
		select {
		case out <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	for s.Scan() {
		line := s.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			kind = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			part := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			eventBytes += len(part)
			if eventBytes > maxBody {
				return errors.New("SSE event exceeds size limit")
			}
			data = append(data, part)
		case line == "":
			if kind != "" || len(data) > 0 {
				if err := send(); err != nil {
					return err
				}
			}
			kind = ""
			data = nil
			eventBytes = 0
		}
	}
	if kind != "" || len(data) > 0 {
		if err := send(); err != nil {
			return err
		}
	}
	return s.Err()
}
func (c *Client) wasManual() bool {
	select {
	case <-c.refresh:
		return true
	default:
		return false
	}
}
func (c *Client) age() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastActivity.IsZero() {
		return 0
	}
	return c.clock.Now().Sub(c.lastActivity)
}

func (c *Client) markActivity() {
	c.mu.Lock()
	c.lastActivity = c.clock.Now()
	c.mu.Unlock()
}

// checkStale is kept small and deterministic for timer-driven tests. The active
// stream loop calls the same thresholds before cancelling and reconnecting.
func (c *Client) checkStale() {
	age := c.age()
	if age >= 60*time.Second {
		c.setStatus(widgetvm.Stale, "数据可能已过期")
	} else if age >= 30*time.Second {
		c.setStatus(widgetvm.Reconnecting, "正在重连")
	}
}
func (c *Client) applyEvent(kind, data string) {
	c.mu.Lock()
	previousObservedAt := c.state.ObservedAt
	c.mu.Unlock()
	c.markActivity()
	var envelope struct {
		ObservedAt string `json:"observedAt"`
		CreatedAt  string `json:"createdAt"`
	}
	_ = json.Unmarshal([]byte(data), &envelope)
	eventTime := envelope.ObservedAt
	if eventTime == "" {
		eventTime = envelope.CreatedAt
	}
	if kind == "heartbeat" {
		c.update(func(s *widgetvm.State) {
			s.Status = widgetvm.Online
			s.ErrorText = ""
		})
		if crossesDate(previousObservedAt, eventTime) {
			c.Refresh()
		}
		return
	}
	var dto response
	switch kind {
	case "status_snapshot":
		if json.Unmarshal([]byte(data), &dto) == nil {
			next := dto.state()
			receivedTrend := next.Trend30d
			c.update(func(s *widgetvm.State) {
				expanded := s.Expanded
				anchorCorner := s.AnchorCorner
				if next.Trend30d == nil {
					next.Trend30d = s.Trend30d
				}
				*s = next
				s.Expanded = expanded
				s.AnchorCorner = anchorCorner
				s.Status = widgetvm.Online
				s.ErrorText = ""
			})
			if trendIsCurrent(receivedTrend, c.clock.Now()) {
				_ = c.trendStore.SaveTrend30d(receivedTrend)
			}
		}
	case "status_quota":
		var x struct {
			Quota *quotaDTO `json:"quota"`
		}
		if json.Unmarshal([]byte(data), &x) == nil {
			c.update(func(s *widgetvm.State) {
				s.Quota = x.Quota.vm()
				s.ObservedAt = envelope.ObservedAt
				s.Status = widgetvm.Online
				s.ErrorText = ""
			})
		}
	case "status_heatmap24h":
		if json.Unmarshal([]byte(data), &dto) == nil {
			c.update(func(s *widgetvm.State) {
				if dto.Heatmap24h != nil {
					s.Heatmap24h = dto.Heatmap24h.vm()
				}
				if dto.Heatmap7d != nil {
					s.Heatmap7d = dto.Heatmap7d.vm()
				}
				if dto.DailyUsage != nil {
					s.Today = dto.DailyUsage.today()
				}
				s.ObservedAt = envelope.ObservedAt
				s.Status = widgetvm.Online
				s.ErrorText = ""
			})
		}
	case "status_errors":
		var x struct {
			Errors []string `json:"errors"`
		}
		if json.Unmarshal([]byte(data), &x) == nil {
			c.update(func(s *widgetvm.State) {
				s.Errors = x.Errors
				s.ObservedAt = envelope.ObservedAt
				s.Status = widgetvm.Online
				s.ErrorText = ""
			})
		}
	}
	// API timezone day changes are a snapshot boundary. Do not synthesize trend data from SSE.
	if crossesDate(previousObservedAt, eventTime) {
		c.Refresh()
	}
}
func crossesDate(previous, next string) bool {
	return datePart(previous) != "" && datePart(next) != "" && datePart(previous) != datePart(next)
}
func datePart(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

var dailyTrendLocation = func() *time.Location {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return location
}()

func expectedTrendEndDate(now time.Time) string {
	return now.In(dailyTrendLocation).AddDate(0, 0, -1).Format("2006-01-02")
}

func trendIsCurrent(trend *widgetvm.Trend30d, now time.Time) bool {
	return trend != nil && trend.EndDate == expectedTrendEndDate(now)
}

func (c *Client) needsDailyTrend30d() bool {
	c.mu.Lock()
	trend := c.state.Trend30d
	c.mu.Unlock()
	return !trendIsCurrent(trend, c.clock.Now())
}

func (c *Client) invalid(text string) { c.setStatus(widgetvm.Invalid, text) }
func (c *Client) connectionUnavailable() {
	c.mu.Lock()
	hasData := !c.lastActivity.IsZero()
	age := time.Duration(0)
	if hasData {
		age = c.clock.Now().Sub(c.lastActivity)
	}
	c.mu.Unlock()
	switch {
	case hasData && age >= 60*time.Second:
		c.setStatus(widgetvm.Stale, "数据可能已过期")
	case hasData:
		c.setStatus(widgetvm.Reconnecting, "正在重连")
	default:
		c.setStatus(widgetvm.Offline, "本机服务未运行")
	}
}
func classifyHTTPStatus(code int) widgetvm.ConnectionStatus {
	if code == http.StatusUnauthorized {
		return widgetvm.Invalid
	}
	return widgetvm.Offline
}
func (c *Client) setStatus(status widgetvm.ConnectionStatus, text string) {
	c.update(func(s *widgetvm.State) {
		if s.Status != widgetvm.Invalid || status == widgetvm.Invalid {
			s.Status = status
			s.ErrorText = text
		}
	})
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
	canonical, err := widgettransport.ParseEndpoint(base)
	if err != nil {
		return nil, errors.New("服务地址无效")
	}
	u, err := url.Parse(canonical)
	if err != nil {
		return nil, errors.New("服务地址无效")
	}
	u.Path = path
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
	HourStart             string `json:"hourStart"`
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	ActiveThreads         int    `json:"activeThreads"`
}

func (h *heatmap24DTO) vm() *widgetvm.Heatmap24h {
	if h == nil {
		return nil
	}
	o := &widgetvm.Heatmap24h{Timezone: h.Timezone}
	for _, b := range h.Buckets {
		o.Buckets = append(o.Buckets, widgetvm.Bucket{HourStart: b.HourStart, InputTokens: b.InputTokens, CachedInputTokens: b.CachedInputTokens, OutputTokens: b.OutputTokens, ReasoningOutputTokens: b.ReasoningOutputTokens, TotalTokens: b.TotalTokens, ActiveThreads: b.ActiveThreads})
	}
	return o
}
func (h *heatmap24DTO) timezone() string {
	if h == nil {
		return ""
	}
	return h.Timezone
}

type heatmap7DTO struct {
	Timezone   string          `json:"timezone"`
	StartDate  string          `json:"startDate"`
	EndDate    string          `json:"endDate"`
	PeakTokens int64           `json:"peakTokens"`
	Days       []heatmapDayDTO `json:"days"`
}
type heatmapDayDTO struct {
	Date        string  `json:"date"`
	TotalTokens int64   `json:"totalTokens"`
	Hours       []int64 `json:"hours"`
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
	InputTokens           int64  `json:"inputTokens"`
	CachedInputTokens     int64  `json:"cachedInputTokens"`
	OutputTokens          int64  `json:"outputTokens"`
	ReasoningOutputTokens int64  `json:"reasoningOutputTokens"`
	TotalTokens           int64  `json:"totalTokens"`
	EstimatedValueLabel   string `json:"estimatedValueLabel"`
}

func (d *dailyDTO) today() *widgetvm.Today {
	if d == nil {
		return nil
	}
	return &widgetvm.Today{InputTokens: d.InputTokens, CachedInputTokens: d.CachedInputTokens, OutputTokens: d.OutputTokens, ReasoningOutputTokens: d.ReasoningOutputTokens, TotalTokens: d.TotalTokens, ValueLabel: d.EstimatedValueLabel}
}

type trendDTO struct {
	Timezone            string        `json:"timezone"`
	StartDate           string        `json:"startDate"`
	EndDate             string        `json:"endDate"`
	TotalTokens         int64         `json:"totalTokens"`
	AverageTokens       int64         `json:"averageTokens"`
	PeakTokens          int64         `json:"peakTokens"`
	EstimatedValueLabel string        `json:"estimatedValueLabel"`
	Days                []trendDayDTO `json:"days"`
}
type trendDayDTO struct {
	Date        string `json:"date"`
	TotalTokens int64  `json:"totalTokens"`
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
