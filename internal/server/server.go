package server

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"openwatcher/internal/buildinfo"
	"openwatcher/internal/config"
	"openwatcher/internal/pairing"
	"openwatcher/internal/quota"
	"openwatcher/internal/sessions"
)

type QuotaSource interface {
	Snapshot() (*quota.Snapshot, []string)
}

type SessionSource interface {
	Snapshot() (sessions.Snapshot, error)
	RolloutPathForThread(threadID string) (string, error)
}

type optionalSessionSnapshotSource interface {
	SnapshotWithOptions(options sessions.SnapshotOptions) (sessions.Snapshot, error)
}

type App struct {
	configPath string
	quota      QuotaSource
	sessions   SessionSource
	clock      func() time.Time
	pricing    *dailyPricingCache

	mu             sync.RWMutex
	cfg            config.Config
	pairingAllowed bool
	pairingSlot    config.PairingSlot
	noAuth         bool

	streamTailInterval         time.Duration
	streamHeartbeatInterval    time.Duration
	statusStreamPollInterval   time.Duration
	contextCompactionLogState  map[string]string
	onStreamExit               func(threadID string)
	onSessionStreamClientEvent func(sessionStreamClientEventLog)
}

func New(configPath string, cfg config.Config, pairingAllowed bool, quotaSource QuotaSource, sessionSource SessionSource) *App {
	cfg.ApplyDefaults()
	return &App{
		configPath:                configPath,
		cfg:                       cfg,
		pairingAllowed:            pairingAllowed,
		pairingSlot:               config.PairingSlotBeta,
		quota:                     quotaSource,
		sessions:                  sessionSource,
		clock:                     time.Now,
		pricing:                   newDailyPricingCache(),
		streamTailInterval:        time.Second,
		streamHeartbeatInterval:   15 * time.Second,
		statusStreamPollInterval:  5 * time.Second,
		contextCompactionLogState: map[string]string{},
	}
}

func (a *App) SetPairingSlot(slot config.PairingSlot) {
	a.mu.Lock()
	a.pairingSlot = config.NormalizePairingSlot(string(slot))
	a.mu.Unlock()
}

func (a *App) SetNoAuth(enabled bool) {
	a.mu.Lock()
	a.noAuth = enabled
	a.mu.Unlock()
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", a.handleHealth)
	mux.HandleFunc("/pair", a.handlePairPage)
	mux.HandleFunc("/api/pair", a.handlePair)
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/status/stream", a.handleStatusStream)
	mux.HandleFunc("/api/session-stream-events", a.handleSessionStreamClientEvent)
	mux.HandleFunc("/api/screenshots", a.handleScreenshotUpload)
	mux.HandleFunc("/api/diagnostics", a.handleDiagnosticUpload)
	mux.HandleFunc("/api/sessions/stream", a.handleSessionWindowStream)
	mux.HandleFunc("/api/sessions/", a.handleSessionRoute)
	mux.HandleFunc("/file/dev/latest.json", a.handleDevLatestAPKMetadata)
	mux.HandleFunc("/file/dev/changelog.json", a.handleDevLatestAPKChangelog)
	mux.HandleFunc("/file/dev/apk", a.handleDevLatestAPK)
	mux.HandleFunc("/file/latest-apk.json", a.handleLatestAPKMetadata)
	mux.HandleFunc("/file/latest-apk-changelog.json", a.handleLatestAPKChangelog)
	mux.HandleFunc("/file/latest-apk", a.handleLatestAPK)
	mux.HandleFunc("/debug/status-local", a.handleLocalDebugStatus)
	return mux
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, a.healthSnapshot())
}

func (a *App) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var request struct {
		DeviceToken string `json:"deviceToken"`
		DeviceName  string `json:"deviceName"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	token := strings.TrimSpace(request.DeviceToken)
	if !pairing.IsUsableToken(token) {
		writeError(w, http.StatusBadRequest, "device token is too short")
		return
	}
	deviceName := strings.TrimSpace(request.DeviceName)
	if len(deviceName) > 80 {
		writeError(w, http.StatusBadRequest, "device name is too long")
		return
	}

	if err := a.savePairing(token, deviceName); err != nil {
		switch err {
		case errPairingDisabled:
			writeError(w, http.StatusForbidden, "pairing not enabled")
		default:
			writeError(w, http.StatusInternalServerError, "failed to save pairing")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handlePairPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(pairPageHTML))
}

var errPairingDisabled = errors.New("pairing disabled")

func (a *App) savePairing(token, deviceName string) error {
	a.mu.RLock()
	allowed := a.pairingAllowed
	nextConfig := a.cfg
	pairingSlot := a.pairingSlot
	a.mu.RUnlock()
	if !allowed {
		return errPairingDisabled
	}

	nextConfig.SetPairingForSlot(
		pairingSlot,
		pairing.HashToken(token),
		deviceName,
		a.clock().Format(time.RFC3339),
	)
	if err := pairing.RecordBinding(
		pairing.HistoryPath(a.configPath),
		pairing.BindingRecord{
			TokenHash:  nextConfig.TokenHashForSlot(pairingSlot),
			DeviceName: nextConfig.PairingForSlot(pairingSlot).DeviceName,
			PairedAt:   nextConfig.PairingForSlot(pairingSlot).PairedAt,
			Source:     "pair-page",
		},
	); err != nil {
		return err
	}

	if err := config.Save(a.configPath, nextConfig); err != nil {
		return err
	}

	a.mu.Lock()
	a.cfg = nextConfig
	a.pairingAllowed = false
	a.mu.Unlock()
	return nil
}

const pairPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <title>OpenWatcher 配对</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #0b1220;
      --panel: #131d31;
      --line: #24324d;
      --text: #eef3ff;
      --muted: #9dafcf;
      --ok: #41d17d;
      --warn: #ffbf47;
      --bad: #ff7474;
      --accent: #4fb3ff;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      background:
        radial-gradient(circle at top, rgba(79,179,255,0.16), transparent 38%),
        linear-gradient(180deg, #0b1220 0%, #070c16 100%);
      color: var(--text);
      font: 16px/1.5 -apple-system, BlinkMacSystemFont, "SF Pro Text", "Helvetica Neue", sans-serif;
      padding: 24px;
    }
    .card {
      width: min(100%, 420px);
      background: rgba(19, 29, 49, 0.96);
      border: 1px solid rgba(79, 179, 255, 0.12);
      border-radius: 20px;
      padding: 24px 20px;
      box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
    }
    h1 {
      margin: 0 0 8px;
      font-size: 24px;
      line-height: 1.2;
    }
    p {
      margin: 0;
      color: var(--muted);
    }
    .status {
      margin-top: 20px;
      padding: 14px 16px;
      border-radius: 16px;
      border: 1px solid var(--line);
      background: rgba(7, 12, 22, 0.7);
    }
    .status-title {
      margin: 0 0 6px;
      font-weight: 600;
      color: var(--text);
    }
    .status-body {
      color: var(--muted);
      white-space: pre-wrap;
    }
    .actions {
      display: flex;
      gap: 12px;
      margin-top: 20px;
    }
    button {
      appearance: none;
      border: 0;
      border-radius: 999px;
      padding: 12px 16px;
      font: inherit;
      font-weight: 600;
      cursor: pointer;
      color: #06101d;
      background: var(--accent);
      flex: 1;
    }
    button.secondary {
      color: var(--text);
      background: rgba(255,255,255,0.08);
      border: 1px solid rgba(255,255,255,0.08);
    }
    button[disabled] {
      opacity: 0.6;
      cursor: default;
    }
    .ok { color: var(--ok); }
    .warn { color: var(--warn); }
    .bad { color: var(--bad); }
    .device {
      margin-top: 18px;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-radius: 999px;
      background: rgba(255,255,255,0.05);
      color: var(--text);
      font-size: 14px;
    }
  </style>
</head>
<body>
  <main class="card">
    <h1>OpenWatcher 配对</h1>
    <p>页面会把手表二维码里的信息提交到当前 watcher 服务，不会在页面内容里回显 device token。</p>
    <div id="device" class="device" hidden></div>
    <section class="status">
      <div id="status-title" class="status-title">准备开始</div>
      <div id="status-body" class="status-body">正在读取二维码参数。</div>
    </section>
    <div class="actions">
      <button id="retry" type="button" disabled>重新提交</button>
      <button id="close" type="button" class="secondary">关闭页面</button>
    </div>
    <noscript>
      <p style="margin-top:16px;color:#ffbf47;">当前浏览器禁用了 JavaScript，无法自动把二维码参数提交到 /api/pair。</p>
    </noscript>
  </main>
  <script>
    (() => {
      const titleEl = document.getElementById("status-title");
      const bodyEl = document.getElementById("status-body");
      const retryEl = document.getElementById("retry");
      const closeEl = document.getElementById("close");
      const deviceEl = document.getElementById("device");

      const params = new URLSearchParams(window.location.search);
      const deviceToken = (params.get("deviceToken") || "").trim();
      const deviceName = (params.get("deviceName") || "watch").trim() || "watch";

      if (window.history && window.history.replaceState) {
        window.history.replaceState(null, "", "/pair");
      }

      if (deviceName) {
        deviceEl.hidden = false;
        deviceEl.textContent = "设备：" + deviceName;
      }

      const setStatus = (kind, title, body) => {
        titleEl.textContent = title;
        titleEl.className = "status-title " + kind;
        bodyEl.textContent = body;
      };

      const submitPair = async () => {
        retryEl.disabled = true;
        if (!deviceToken) {
          setStatus("bad", "二维码无效", "缺少 deviceToken。请回到手表重新生成二维码后再扫码。");
          return;
        }

        setStatus("warn", "正在提交", "正在把 " + deviceName + " 的配对请求发送给 watcher 服务。");
        try {
          const response = await fetch("/api/pair", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ deviceToken, deviceName }),
          });
          let payload = {};
          try {
            payload = await response.json();
          } catch (_) {}

          if (response.ok && payload.ok !== false) {
            setStatus("ok", "配对成功", "服务端已保存手表 token 哈希。可以回到手表，等待下一次状态轮询。");
            return;
          }

          const errorText = payload.error || ("HTTP " + response.status);
          if (response.status === 403) {
            setStatus("bad", "当前不能配对", "服务端未开启配对模式。请先让 OpenWatcher 以首次配对或重新配对模式运行。");
          } else if (response.status === 400) {
            setStatus("bad", "二维码参数无效", errorText);
          } else {
            setStatus("bad", "配对失败", errorText);
          }
        } catch (error) {
          setStatus("bad", "服务不可达", "当前页面无法访问 /api/pair，请检查 tunnel 或本地 watcher 服务。");
        } finally {
          retryEl.disabled = false;
        }
      };

      retryEl.addEventListener("click", submitPair);
      closeEl.addEventListener("click", () => window.close());
      submitPair();
    })();
  </script>
</body>
</html>
`

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	response := a.buildStatusResponse(statusResponseOptions{
		IncludeDailyTrend30d: parseBoolQuery(r.URL.Query().Get("includeDailyTrend30d")),
	})
	writeJSON(w, http.StatusOK, response)
}

func (a *App) handleStatusStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unavailable")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	current := a.buildStatusResponse(statusResponseOptions{
		IncludeDailyTrend30d: parseBoolQuery(r.URL.Query().Get("includeDailyTrend30d")),
	})
	fingerprints := statusResponseFingerprints(current)
	writeStatusSnapshotEvent(w, flusher, current)

	pollInterval := a.statusStreamPollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	heartbeatInterval := a.streamHeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 15 * time.Second
	}
	pollTicker := time.NewTicker(pollInterval)
	defer pollTicker.Stop()
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-pollTicker.C:
			next := a.buildStatusResponse(statusResponseOptions{})
			nextFingerprints := statusResponseFingerprints(next)
			if nextFingerprints.Quota != fingerprints.Quota {
				writeStatusQuotaEvent(w, flusher, next)
			}
			if nextFingerprints.Sessions != fingerprints.Sessions {
				writeStatusSessionsEvent(w, flusher, next)
			}
			if nextFingerprints.Heatmap24h != fingerprints.Heatmap24h ||
				nextFingerprints.Heatmap7d != fingerprints.Heatmap7d ||
				nextFingerprints.DailyUsage != fingerprints.DailyUsage {
				writeStatusHeatmapEvent(w, flusher, next)
			}
			if nextFingerprints.Errors != fingerprints.Errors {
				writeStatusErrorsEvent(w, flusher, next)
			}
			fingerprints = nextFingerprints
		case <-heartbeatTicker.C:
			writeStatusHeartbeatEvent(w, flusher, a.clock())
		}
	}
}

func (a *App) handleSessionRoute(w http.ResponseWriter, r *http.Request) {
	threadID, ok := parseSessionStreamPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	a.handleSessionStream(w, r, threadID)
}

func parseSessionStreamPath(path string) (string, bool) {
	rest := strings.TrimPrefix(path, "/api/sessions/")
	if rest == path || !strings.HasSuffix(rest, "/stream") {
		return "", false
	}
	threadID := strings.TrimSuffix(rest, "/stream")
	threadID = strings.Trim(threadID, "/")
	return threadID, threadID != ""
}

func (a *App) handleSessionStream(w http.ResponseWriter, r *http.Request, threadID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rolloutPath, err := a.sessions.RolloutPathForThread(threadID)
	if err != nil {
		if errors.Is(err, sessions.ErrThreadNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "sessions unavailable")
		return
	}
	if rolloutPath == "" {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if info, err := os.Stat(rolloutPath); err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "rollout unavailable")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unavailable")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	defer func() {
		if a.onStreamExit != nil {
			a.onStreamExit(threadID)
		}
	}()

	includeMessages := parseIncludeMessages(r.URL.Query().Get("includeMessages"))
	runtimeState, offset, runtimeMachine, err := sessions.RecoverRuntimeState(rolloutPath, threadID)
	if err != nil {
		writeStreamError(w, flusher, threadID, a.clock(), "rollout unavailable")
		return
	}
	writeRuntimeStateEvent(w, flusher, runtimeState)

	if includeMessages {
		latest, _, ok, err := sessions.LatestAgentMessage(rolloutPath, threadID)
		if err != nil {
			writeStreamError(w, flusher, threadID, a.clock(), "rollout unavailable")
			return
		}
		if ok {
			writeAgentMessageEvent(w, flusher, latest)
		}
	}
	if runtimeState.Lifecycle == sessions.RuntimeLifecycleIdle && !includeMessages {
		writeHeartbeatEvent(w, flusher, threadID, a.clock())
	}

	tailInterval := a.streamTailInterval
	if tailInterval <= 0 {
		tailInterval = time.Second
	}
	heartbeatInterval := a.streamHeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 15 * time.Second
	}

	tailTicker := time.NewTicker(tailInterval)
	defer tailTicker.Stop()
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-tailTicker.C:
			updates, nextOffset, err := sessions.ReadStreamUpdatesFromOffset(rolloutPath, threadID, offset, runtimeMachine)
			if err != nil {
				writeStreamError(w, flusher, threadID, a.clock(), "rollout unavailable")
				return
			}
			offset = nextOffset
			for _, update := range updates {
				if update.RuntimeState != nil {
					writeRuntimeStateEvent(w, flusher, *update.RuntimeState)
				}
				if includeMessages && update.AgentMessage != nil {
					writeAgentMessageEvent(w, flusher, *update.AgentMessage)
				}
			}
		case <-heartbeatTicker.C:
			writeHeartbeatEvent(w, flusher, threadID, a.clock())
		}
	}
}

func parseIncludeMessages(value string) bool {
	switch strings.TrimSpace(value) {
	case "0", "false", "no":
		return false
	default:
		return true
	}
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (a *App) handleSessionStreamClientEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !a.authorized(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var request sessionStreamClientEventRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := request.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	event := sessionStreamClientEventLog{
		ReceivedAt:           a.clock().Format(time.RFC3339),
		EventType:            normalizeSessionStreamClientEventType(request.EventType),
		ThreadID:             request.ThreadID,
		DeviceName:           request.DeviceName,
		AppVersion:           request.AppVersion,
		ReconnectAttempt:     request.ReconnectAttempt,
		Reason:               request.Reason,
		Detail:               request.Detail,
		StatusCode:           request.StatusCode,
		Retryable:            request.Retryable,
		ConnectedMs:          request.ConnectedMs,
		NextRetryDelayMs:     request.NextRetryDelayMs,
		ReceivedAgentMessage: request.ReceivedAgentMessage,
		FirstEventType:       request.FirstEventType,
	}
	a.logSessionStreamClientEvent(event)
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) handleLocalDebugStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !requestFromLoopback(r) {
		writeError(w, http.StatusForbidden, "loopback only")
		return
	}

	response := a.buildStatusResponse(statusResponseOptions{})
	writeJSON(w, http.StatusOK, response)
}

type statusResponseOptions struct {
	IncludeDailyTrend30d bool
}

func (a *App) buildStatusResponse(options statusResponseOptions) statusResponse {
	quotaSnapshot, errors := a.quota.Snapshot()
	if quotaSnapshot == nil {
		quotaSnapshot = &quota.Snapshot{Source: "oauth-api", Fresh: false, Status: quota.StatusUnavailable}
	} else if quotaSnapshot.Status == "" {
		switch {
		case quotaSnapshot.Fresh:
			quotaSnapshot.Status = quota.StatusOK
		case quotaSnapshot.FiveHour != nil || quotaSnapshot.Weekly != nil || quotaSnapshot.PlanType != "":
			quotaSnapshot.Status = quota.StatusStale
		default:
			quotaSnapshot.Status = quota.StatusUnavailable
		}
	}
	errors = nil

	sessionSnapshot, err := a.sessionSnapshot(options)
	if err != nil {
		errors = append(errors, "sessions unavailable")
	}
	if errors == nil {
		errors = []string{}
	}

	var heatmap24h *sessions.Heatmap24hSnapshot
	var heatmap7d *sessions.Heatmap7dSnapshot
	var dailyUsage *dailyUsageResponse
	var dailyTrend30d *dailyTrend30dResponse
	if err == nil {
		heatmap24h = &sessionSnapshot.Heatmap24h
		heatmap7d = &sessionSnapshot.Heatmap7d
		pricing, pricingErr := a.pricing.snapshotForDay(a.clock())
		response := buildDailyUsageResponse(sessionSnapshot.DailyUsage, pricing, pricingErr)
		dailyUsage = &response
		if sessionSnapshot.DailyTrend30d != nil {
			trendResponse := buildDailyTrend30dResponse(*sessionSnapshot.DailyTrend30d, pricing, pricingErr)
			dailyTrend30d = &trendResponse
		}
	}

	response := statusResponse{
		OK:            true,
		ObservedAt:    a.clock().Format(time.RFC3339),
		Quota:         quotaSnapshot,
		Heatmap24h:    heatmap24h,
		Heatmap7d:     heatmap7d,
		DailyUsage:    dailyUsage,
		DailyTrend30d: dailyTrend30d,
		Sessions:      sessionSnapshot.Sessions,
		Errors:        errors,
	}
	a.logContextCompactionTransitions(response.Sessions)
	return response
}

func (a *App) logContextCompactionTransitions(input []sessions.SessionSnapshot) {
	next := make(map[string]string)
	byThreadID := make(map[string]*sessions.ContextCompactionSnapshot)
	for _, session := range input {
		if session.ContextCompaction == nil {
			continue
		}
		signature := contextCompactionSignature(session.ContextCompaction)
		next[session.ThreadID] = signature
		byThreadID[session.ThreadID] = session.ContextCompaction
	}

	a.mu.Lock()
	previous := a.contextCompactionLogState
	if previous == nil {
		previous = map[string]string{}
	}
	a.contextCompactionLogState = next
	a.mu.Unlock()

	for threadID, signature := range next {
		if previous[threadID] == signature {
			continue
		}
		compaction := byThreadID[threadID]
		log.Printf(
			"context_compaction_detected threadId=%s trigger=%s turnId=%s startedAt=%s updatedAt=%s",
			threadID,
			compaction.Trigger,
			compaction.TurnID,
			compaction.StartedAt.Format(time.RFC3339Nano),
			compaction.UpdatedAt.Format(time.RFC3339Nano),
		)
	}
	for threadID, signature := range previous {
		if _, ok := next[threadID]; ok {
			continue
		}
		log.Printf("context_compaction_cleared threadId=%s previous=%s", threadID, signature)
	}
}

func contextCompactionSignature(compaction *sessions.ContextCompactionSnapshot) string {
	if compaction == nil {
		return ""
	}
	return strings.Join([]string{
		compaction.Trigger,
		compaction.TurnID,
		compaction.StartedAt.Format(time.RFC3339Nano),
		compaction.UpdatedAt.Format(time.RFC3339Nano),
	}, "|")
}

func (a *App) sessionSnapshot(options statusResponseOptions) (sessions.Snapshot, error) {
	if options.IncludeDailyTrend30d {
		if source, ok := a.sessions.(optionalSessionSnapshotSource); ok {
			return source.SnapshotWithOptions(sessions.SnapshotOptions{
				IncludeDailyTrend30d: true,
			})
		}
	}
	return a.sessions.Snapshot()
}

func (a *App) logSessionStreamClientEvent(event sessionStreamClientEventLog) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	log.Printf("watcher_session_stream_client_event %s", data)
	if a.onSessionStreamClientEvent != nil {
		a.onSessionStreamClientEvent(event)
	}
}

func (a *App) authorized(r *http.Request) bool {
	a.mu.RLock()
	noAuth := a.noAuth
	tokenHash := a.cfg.TokenHashForSlot(a.pairingSlot)
	a.mu.RUnlock()
	if noAuth {
		return true
	}
	token := watcherHeaderValue(r.Header, "Token")
	return pairing.VerifyTokenHash(token, tokenHash)
}

func requestFromLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

type statusResponse struct {
	OK            bool                         `json:"ok"`
	ObservedAt    string                       `json:"observedAt"`
	Quota         *quota.Snapshot              `json:"quota"`
	Heatmap24h    *sessions.Heatmap24hSnapshot `json:"heatmap24h,omitempty"`
	Heatmap7d     *sessions.Heatmap7dSnapshot  `json:"heatmap7d,omitempty"`
	DailyUsage    *dailyUsageResponse          `json:"dailyUsage,omitempty"`
	DailyTrend30d *dailyTrend30dResponse       `json:"dailyTrend30d,omitempty"`
	Sessions      []sessions.SessionSnapshot   `json:"sessions"`
	Errors        []string                     `json:"errors"`
}

type statusSnapshotEvent struct {
	Type string `json:"type"`
	statusResponse
}

type statusQuotaEvent struct {
	Type       string          `json:"type"`
	ObservedAt string          `json:"observedAt"`
	Quota      *quota.Snapshot `json:"quota"`
}

type statusHeatmapEvent struct {
	Type       string                       `json:"type"`
	ObservedAt string                       `json:"observedAt"`
	Heatmap24h *sessions.Heatmap24hSnapshot `json:"heatmap24h,omitempty"`
	Heatmap7d  *sessions.Heatmap7dSnapshot  `json:"heatmap7d,omitempty"`
	DailyUsage *dailyUsageResponse          `json:"dailyUsage,omitempty"`
}

type statusSessionsEvent struct {
	Type       string                     `json:"type"`
	ObservedAt string                     `json:"observedAt"`
	Sessions   []sessions.SessionSnapshot `json:"sessions"`
}

type statusErrorsEvent struct {
	Type       string   `json:"type"`
	ObservedAt string   `json:"observedAt"`
	Errors     []string `json:"errors"`
}

type statusHeartbeatEvent struct {
	Type      string `json:"type"`
	CreatedAt string `json:"createdAt"`
}

type statusFingerprints struct {
	Quota      string
	Heatmap24h string
	Heatmap7d  string
	DailyUsage string
	Sessions   string
	Errors     string
}

type healthResponse struct {
	OK     bool                 `json:"ok"`
	Build  buildinfo.Info       `json:"build"`
	Config healthConfigResponse `json:"config"`
	Codex  healthCodexResponse  `json:"codex"`
}

type healthConfigResponse struct {
	Listen        string `json:"listen"`
	PublicBaseURL string `json:"publicBaseUrl"`
	PairingSlot   string `json:"pairingSlot"`
	Paired        bool   `json:"paired"`
	NoAuth        bool   `json:"noAuth"`
}

type healthCodexResponse struct {
	HomeDetected     bool `json:"homeDetected"`
	AuthDetected     bool `json:"authDetected"`
	SessionsDetected bool `json:"sessionsDetected"`
}

type heartbeatEvent struct {
	Type      string `json:"type"`
	ThreadID  string `json:"threadId"`
	CreatedAt string `json:"createdAt"`
}

type streamErrorEvent struct {
	Type      string `json:"type"`
	ThreadID  string `json:"threadId"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

type sessionStreamClientEventRequest struct {
	EventType            string `json:"eventType"`
	ThreadID             string `json:"threadId"`
	DeviceName           string `json:"deviceName"`
	AppVersion           string `json:"appVersion"`
	ReconnectAttempt     int    `json:"reconnectAttempt"`
	Reason               string `json:"reason,omitempty"`
	Detail               string `json:"detail,omitempty"`
	StatusCode           int    `json:"statusCode,omitempty"`
	Retryable            *bool  `json:"retryable,omitempty"`
	ConnectedMs          int64  `json:"connectedMs,omitempty"`
	NextRetryDelayMs     int64  `json:"nextRetryDelayMs,omitempty"`
	ReceivedAgentMessage bool   `json:"receivedAgentMessage,omitempty"`
	FirstEventType       string `json:"firstEventType,omitempty"`
}

func (a *App) healthSnapshot() healthResponse {
	a.mu.RLock()
	cfg := a.cfg
	noAuth := a.noAuth
	pairingSlot := a.pairingSlot
	a.mu.RUnlock()

	activePairing := cfg.PairingForSlot(pairingSlot)
	return healthResponse{
		OK:    true,
		Build: buildinfo.Current(),
		Config: healthConfigResponse{
			Listen:        cfg.Listen,
			PublicBaseURL: cfg.EffectivePublicBaseURL(),
			PairingSlot:   string(pairingSlot),
			Paired:        strings.TrimSpace(activePairing.TokenHash) != "",
			NoAuth:        noAuth,
		},
		Codex: detectCodexStatus(cfg.CodexHome),
	}
}

func detectCodexStatus(configuredHome string) healthCodexResponse {
	resolvedHome, err := config.ResolveCodexHome(configuredHome)
	if err != nil {
		return healthCodexResponse{}
	}

	authInfo, authErr := os.Stat(filepath.Join(resolvedHome, "auth.json"))
	sessionsInfo, sessionsErr := os.Stat(filepath.Join(resolvedHome, "sessions"))

	return healthCodexResponse{
		HomeDetected:     true,
		AuthDetected:     authErr == nil && !authInfo.IsDir(),
		SessionsDetected: sessionsErr == nil && sessionsInfo.IsDir(),
	}
}

func (r sessionStreamClientEventRequest) validate() error {
	eventType := strings.TrimSpace(r.EventType)
	if eventType != "disconnect" && eventType != "reconnectsuccess" && eventType != "reconnect_success" {
		return errors.New("invalid event type")
	}
	threadID := strings.TrimSpace(r.ThreadID)
	if threadID == "" {
		return errors.New("threadId is required")
	}
	if len(threadID) > 128 {
		return errors.New("threadId is too long")
	}
	if len(strings.TrimSpace(r.DeviceName)) > 80 {
		return errors.New("deviceName is too long")
	}
	if len(strings.TrimSpace(r.AppVersion)) > 40 {
		return errors.New("appVersion is too long")
	}
	if len(strings.TrimSpace(r.Reason)) > 40 {
		return errors.New("reason is too long")
	}
	if len(strings.TrimSpace(r.Detail)) > 240 {
		return errors.New("detail is too long")
	}
	if len(strings.TrimSpace(r.FirstEventType)) > 32 {
		return errors.New("firstEventType is too long")
	}
	if r.ReconnectAttempt < 0 {
		return errors.New("reconnectAttempt must be non-negative")
	}
	if r.ConnectedMs < 0 {
		return errors.New("connectedMs must be non-negative")
	}
	if r.NextRetryDelayMs < 0 {
		return errors.New("nextRetryDelayMs must be non-negative")
	}
	return nil
}

func normalizeSessionStreamClientEventType(value string) string {
	switch strings.TrimSpace(value) {
	case "reconnectsuccess":
		return "reconnect_success"
	default:
		return strings.TrimSpace(value)
	}
}

type sessionStreamClientEventLog struct {
	ReceivedAt           string `json:"receivedAt"`
	EventType            string `json:"eventType"`
	ThreadID             string `json:"threadId"`
	DeviceName           string `json:"deviceName,omitempty"`
	AppVersion           string `json:"appVersion,omitempty"`
	ReconnectAttempt     int    `json:"reconnectAttempt,omitempty"`
	Reason               string `json:"reason,omitempty"`
	Detail               string `json:"detail,omitempty"`
	StatusCode           int    `json:"statusCode,omitempty"`
	Retryable            *bool  `json:"retryable,omitempty"`
	ConnectedMs          int64  `json:"connectedMs,omitempty"`
	NextRetryDelayMs     int64  `json:"nextRetryDelayMs,omitempty"`
	ReceivedAgentMessage bool   `json:"receivedAgentMessage,omitempty"`
	FirstEventType       string `json:"firstEventType,omitempty"`
}

func statusResponseFingerprints(response statusResponse) statusFingerprints {
	return statusFingerprints{
		Quota:      stableJSON(response.Quota),
		Heatmap24h: stableJSON(fingerprintHeatmap(response.Heatmap24h)),
		Heatmap7d:  stableJSON(fingerprintHeatmap7d(response.Heatmap7d)),
		DailyUsage: stableJSON(fingerprintDailyUsage(response.DailyUsage)),
		Sessions:   stableJSON(fingerprintSessions(response.Sessions)),
		Errors:     stableJSON(response.Errors),
	}
}

type heatmapFingerprint struct {
	Timezone      string                   `json:"timezone"`
	PeakHourStart *time.Time               `json:"peakHourStart,omitempty"`
	Buckets       []sessions.HeatmapBucket `json:"buckets"`
}

func fingerprintHeatmap(input *sessions.Heatmap24hSnapshot) *heatmapFingerprint {
	if input == nil {
		return nil
	}
	return &heatmapFingerprint{
		Timezone:      input.Timezone,
		PeakHourStart: input.PeakHourStart,
		Buckets:       input.Buckets,
	}
}

type heatmap7dFingerprint struct {
	Timezone   string                  `json:"timezone"`
	StartDate  string                  `json:"startDate"`
	EndDate    string                  `json:"endDate"`
	PeakTokens int64                   `json:"peakTokens"`
	Days       []sessions.Heatmap7dDay `json:"days"`
}

func fingerprintHeatmap7d(input *sessions.Heatmap7dSnapshot) *heatmap7dFingerprint {
	if input == nil {
		return nil
	}
	return &heatmap7dFingerprint{
		Timezone:   input.Timezone,
		StartDate:  input.StartDate,
		EndDate:    input.EndDate,
		PeakTokens: input.PeakTokens,
		Days:       input.Days,
	}
}

type dailyUsageFingerprint struct {
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

func fingerprintDailyUsage(input *dailyUsageResponse) *dailyUsageFingerprint {
	if input == nil {
		return nil
	}
	return &dailyUsageFingerprint{
		TotalTokens:              input.TotalTokens,
		InputTokens:              input.InputTokens,
		CachedInputTokens:        input.CachedInputTokens,
		OutputTokens:             input.OutputTokens,
		ReasoningOutputTokens:    input.ReasoningOutputTokens,
		ActiveSessions:           input.ActiveSessions,
		EstimatedValueUSD:        input.EstimatedValueUSD,
		EstimatedValueLabel:      input.EstimatedValueLabel,
		PricingDate:              input.PricingDate,
		PricingSourceURL:         input.PricingSourceURL,
		PricingSource:            input.PricingSource,
		PricingUnavailableReason: input.PricingUnavailableReason,
		ModelShares:              input.ModelShares,
	}
}

type sessionFingerprint struct {
	ThreadID                       string                              `json:"threadId"`
	Title                          string                              `json:"title"`
	UpdatedAt                      time.Time                           `json:"updatedAt"`
	Model                          string                              `json:"model"`
	ReasoningEffort                string                              `json:"reasoningEffort,omitempty"`
	TokensUsedTotal                int64                               `json:"tokensUsedTotal"`
	ContextUsedTokens              int64                               `json:"contextUsedTokens"`
	ContextWindow                  int64                               `json:"contextWindow"`
	ContextPressurePercent         int                                 `json:"contextPressurePercent"`
	ContextCompactThresholdTokens  int64                               `json:"contextCompactThresholdTokens,omitempty"`
	ContextCompactThresholdPercent int                                 `json:"contextCompactThresholdPercent,omitempty"`
	ContextCompaction              *sessions.ContextCompactionSnapshot `json:"contextCompaction,omitempty"`
}

func fingerprintSessions(input []sessions.SessionSnapshot) []sessionFingerprint {
	output := make([]sessionFingerprint, 0, len(input))
	for _, session := range input {
		output = append(output, sessionFingerprint{
			ThreadID:                       session.ThreadID,
			Title:                          session.Title,
			UpdatedAt:                      session.UpdatedAt,
			Model:                          session.Model,
			ReasoningEffort:                session.ReasoningEffort,
			TokensUsedTotal:                session.TokensUsedTotal,
			ContextUsedTokens:              session.ContextUsedTokens,
			ContextWindow:                  session.ContextWindow,
			ContextPressurePercent:         session.ContextPressurePercent,
			ContextCompactThresholdTokens:  session.ContextCompactThresholdTokens,
			ContextCompactThresholdPercent: session.ContextCompactThresholdPercent,
			ContextCompaction:              session.ContextCompaction,
		})
	}
	return output
}

func stableJSON(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}

func writeStatusSnapshotEvent(w http.ResponseWriter, flusher http.Flusher, response statusResponse) {
	writeSSE(w, flusher, "status_snapshot", statusEventID("status_snapshot", response.ObservedAt), statusSnapshotEvent{
		Type:           "status_snapshot",
		statusResponse: response,
	})
}

func writeStatusQuotaEvent(w http.ResponseWriter, flusher http.Flusher, response statusResponse) {
	writeSSE(w, flusher, "status_quota", statusEventID("status_quota", response.ObservedAt), statusQuotaEvent{
		Type:       "status_quota",
		ObservedAt: response.ObservedAt,
		Quota:      response.Quota,
	})
}

func writeStatusHeatmapEvent(w http.ResponseWriter, flusher http.Flusher, response statusResponse) {
	writeSSE(w, flusher, "status_heatmap24h", statusEventID("status_heatmap24h", response.ObservedAt), statusHeatmapEvent{
		Type:       "status_heatmap24h",
		ObservedAt: response.ObservedAt,
		Heatmap24h: response.Heatmap24h,
		Heatmap7d:  response.Heatmap7d,
		DailyUsage: response.DailyUsage,
	})
}

func writeStatusSessionsEvent(w http.ResponseWriter, flusher http.Flusher, response statusResponse) {
	writeSSE(w, flusher, "status_sessions", statusEventID("status_sessions", response.ObservedAt), statusSessionsEvent{
		Type:       "status_sessions",
		ObservedAt: response.ObservedAt,
		Sessions:   response.Sessions,
	})
}

func writeStatusErrorsEvent(w http.ResponseWriter, flusher http.Flusher, response statusResponse) {
	writeSSE(w, flusher, "status_errors", statusEventID("status_errors", response.ObservedAt), statusErrorsEvent{
		Type:       "status_errors",
		ObservedAt: response.ObservedAt,
		Errors:     response.Errors,
	})
}

func writeStatusHeartbeatEvent(w http.ResponseWriter, flusher http.Flusher, now time.Time) {
	createdAt := now.Format(time.RFC3339)
	writeSSE(w, flusher, "heartbeat", statusEventID("status_heartbeat", createdAt), statusHeartbeatEvent{
		Type:      "heartbeat",
		CreatedAt: createdAt,
	})
}

func statusEventID(prefix, timestamp string) string {
	return prefix + "-" + strings.ReplaceAll(timestamp, ":", "")
}

func writeAgentMessageEvent(w http.ResponseWriter, flusher http.Flusher, message sessions.AgentMessage) {
	writeSSE(w, flusher, "agent_message", message.EventID, message)
}

func writeRuntimeStateEvent(w http.ResponseWriter, flusher http.Flusher, state sessions.RuntimeState) {
	eventID := "runtime-" + state.ThreadID + "-" + state.TurnID + "-" + strconv.FormatInt(state.Sequence, 10)
	writeSSE(w, flusher, "runtime_state", eventID, state)
}

func writeHeartbeatEvent(w http.ResponseWriter, flusher http.Flusher, threadID string, now time.Time) {
	eventID := "heartbeat-" + now.Format("20060102150405.000000000")
	writeSSE(w, flusher, "heartbeat", eventID, heartbeatEvent{
		Type:      "heartbeat",
		ThreadID:  threadID,
		CreatedAt: now.Format(time.RFC3339),
	})
}

func writeStreamError(w http.ResponseWriter, flusher http.Flusher, threadID string, now time.Time, message string) {
	eventID := "error-" + now.Format("20060102150405.000000000")
	writeSSE(w, flusher, "error", eventID, streamErrorEvent{
		Type:      "error",
		ThreadID:  threadID,
		Message:   message,
		CreatedAt: now.Format(time.RFC3339),
	})
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, eventName, eventID string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write([]byte("event: " + eventName + "\n"))
	if eventID != "" {
		_, _ = w.Write([]byte("id: " + eventID + "\n"))
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n\n"))
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"ok":    false,
		"error": message,
	})
}
