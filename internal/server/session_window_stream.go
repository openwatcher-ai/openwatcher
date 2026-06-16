package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"openwatcher/internal/sessions"
)

const sessionWindowStreamLimit = 5

type sessionWindowStreamState struct {
	observedAt string
	order      []string
	entries    map[string]*sessionWindowStreamEntry
}

type sessionWindowStreamEntry struct {
	snapshot       sessions.SessionSnapshot
	runtimeState   sessions.RuntimeState
	latestMessage  *sessions.AgentMessage
	rolloutPath    string
	offset         int64
	runtimeMachine *sessions.RuntimeStateMachine
}

type sessionWindowEvent struct {
	Type        string                     `json:"type"`
	ObservedAt  string                     `json:"observedAt"`
	Limit       int                        `json:"limit"`
	ThreadOrder []string                   `json:"threadOrder"`
	Sessions    []sessionWindowSessionData `json:"sessions"`
}

type sessionWindowSessionData struct {
	ThreadID                       string                              `json:"threadId"`
	Title                          string                              `json:"title"`
	Model                          string                              `json:"model"`
	ReasoningEffort                string                              `json:"reasoningEffort,omitempty"`
	TokensUsedTotal                int64                               `json:"tokensUsedTotal"`
	ContextUsedTokens              int64                               `json:"contextUsedTokens"`
	ContextWindow                  int64                               `json:"contextWindow"`
	ContextPressurePercent         int                                 `json:"contextPressurePercent"`
	ContextCompactThresholdTokens  int64                               `json:"contextCompactThresholdTokens,omitempty"`
	ContextCompactThresholdPercent int                                 `json:"contextCompactThresholdPercent,omitempty"`
	ContextCompaction              *sessions.ContextCompactionSnapshot `json:"contextCompaction,omitempty"`
	LastActiveAgoMinutes           int64                               `json:"lastActiveAgoMinutes"`
	RuntimeState                   sessions.RuntimeState               `json:"runtimeState"`
	LatestAgentMessage             *sessions.AgentMessage              `json:"latestAgentMessage,omitempty"`
}

func (a *App) handleSessionWindowStream(w http.ResponseWriter, r *http.Request) {
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

	limit := parseSessionWindowLimit(r.URL.Query().Get("limit"))
	preferredOrder := parsePreferredThreadOrder(r.URL.Query().Get("preferredOrder"))
	state, err := a.buildSessionWindowStreamState(limit, preferredOrder, nil)
	if err != nil {
		logSessionWindowStreamSummary("window_build_failed", map[string]any{
			"stage":          "initial",
			"limit":          limit,
			"preferredOrder": preferredOrder,
			"message":        "sessions unavailable",
		})
		writeWindowStreamError(w, flusher, a.clock(), "sessions unavailable")
		return
	}
	logSessionWindowStreamSummary("window_initialized", map[string]any{
		"limit":          limit,
		"preferredOrder": preferredOrder,
		"threadOrder":    state.order,
		"activeCount":    countActiveWindowEntries(state.entries, state.order),
	})
	writeSessionWindowEvent(w, flusher, state)

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
			if err := a.tailSessionWindowEntries(w, flusher, state); err != nil {
				logSessionWindowStreamSummary("window_tail_failed", map[string]any{
					"threadOrder": state.order,
					"message":     "rollout unavailable",
				})
				writeWindowStreamError(w, flusher, a.clock(), "rollout unavailable")
				return
			}

			nextState, err := a.buildSessionWindowStreamState(limit, state.order, state.entries)
			if err != nil {
				logSessionWindowStreamSummary("window_build_failed", map[string]any{
					"stage":          "refresh",
					"limit":          limit,
					"preferredOrder": state.order,
					"message":        "sessions unavailable",
				})
				writeWindowStreamError(w, flusher, a.clock(), "sessions unavailable")
				return
			}
			if !sessionWindowStatesEqual(state, nextState) {
				insertedThreadIDs := diffThreadIDs(nextState.order, state.order)
				evictedThreadIDs := diffThreadIDs(state.order, nextState.order)
				previousActiveCount := countActiveWindowEntries(state.entries, state.order)
				nextActiveCount := countActiveWindowEntries(nextState.entries, nextState.order)
				if len(insertedThreadIDs) > 0 || len(evictedThreadIDs) > 0 || previousActiveCount != nextActiveCount || !slices.Equal(state.order, nextState.order) {
					logSessionWindowStreamSummary("window_changed", map[string]any{
						"previousOrder":     state.order,
						"nextOrder":         nextState.order,
						"insertedThreadIds": insertedThreadIDs,
						"evictedThreadIds":  evictedThreadIDs,
						"activeCount":       nextActiveCount,
					})
				}
				writeSessionWindowEvent(w, flusher, nextState)
			}
			state = nextState
		case <-heartbeatTicker.C:
			writeWindowHeartbeatEvent(w, flusher, a.clock())
		}
	}
}

func parseSessionWindowLimit(value string) int {
	if strings.TrimSpace(value) == "" {
		return sessionWindowStreamLimit
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return sessionWindowStreamLimit
	}
	if parsed > sessionWindowStreamLimit {
		return sessionWindowStreamLimit
	}
	return parsed
}

func parsePreferredThreadOrder(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	order := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		threadID := strings.TrimSpace(part)
		if threadID == "" || seen[threadID] {
			continue
		}
		seen[threadID] = true
		order = append(order, threadID)
	}
	return order
}

func (a *App) buildSessionWindowStreamState(
	limit int,
	previousOrder []string,
	previousEntries map[string]*sessionWindowStreamEntry,
) (sessionWindowStreamState, error) {
	snapshot, err := a.sessions.Snapshot()
	if err != nil {
		return sessionWindowStreamState{}, err
	}
	observation := a.clock().Format(time.RFC3339)
	limit = max(1, min(limit, sessionWindowStreamLimit))
	candidates := snapshot.Sessions
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	entries := map[string]*sessionWindowStreamEntry{}
	for _, session := range candidates {
		entry, err := a.sessionWindowEntryForSnapshot(session, previousEntries)
		if err != nil {
			continue
		}
		entries[session.ThreadID] = entry
	}

	order := computeSessionWindowOrder(candidates, entries, previousOrder, limit)
	trimmedEntries := make(map[string]*sessionWindowStreamEntry, len(order))
	for _, threadID := range order {
		if entry := entries[threadID]; entry != nil {
			trimmedEntries[threadID] = entry
		}
	}

	return sessionWindowStreamState{
		observedAt: observation,
		order:      order,
		entries:    trimmedEntries,
	}, nil
}

func (a *App) sessionWindowEntryForSnapshot(
	session sessions.SessionSnapshot,
	previousEntries map[string]*sessionWindowStreamEntry,
) (*sessionWindowStreamEntry, error) {
	if previousEntries != nil {
		if previous := previousEntries[session.ThreadID]; previous != nil {
			cloned := *previous
			cloned.snapshot = session
			return &cloned, nil
		}
	}

	rolloutPath, err := a.sessions.RolloutPathForThread(session.ThreadID)
	if err != nil || rolloutPath == "" {
		return nil, err
	}
	if info, err := os.Stat(rolloutPath); err != nil || info.IsDir() {
		return nil, err
	}
	runtimeState, offset, runtimeMachine, err := sessions.RecoverRuntimeState(rolloutPath, session.ThreadID)
	if err != nil {
		return nil, err
	}
	var latestMessage *sessions.AgentMessage
	if message, _, ok, err := sessions.LatestAgentMessage(rolloutPath, session.ThreadID); err == nil && ok {
		copied := message
		latestMessage = &copied
	}
	return &sessionWindowStreamEntry{
		snapshot:       session,
		runtimeState:   runtimeState,
		latestMessage:  latestMessage,
		rolloutPath:    rolloutPath,
		offset:         offset,
		runtimeMachine: runtimeMachine,
	}, nil
}

func computeSessionWindowOrder(
	candidates []sessions.SessionSnapshot,
	entries map[string]*sessionWindowStreamEntry,
	previousOrder []string,
	limit int,
) []string {
	if len(candidates) == 0 {
		return nil
	}

	candidateOrder := make([]string, 0, len(candidates))
	candidateByThreadID := make(map[string]sessions.SessionSnapshot, len(candidates))
	activeByThreadID := make(map[string]bool, len(candidates))
	for _, session := range candidates {
		if entries[session.ThreadID] == nil {
			continue
		}
		candidateOrder = append(candidateOrder, session.ThreadID)
		candidateByThreadID[session.ThreadID] = session
		activeByThreadID[session.ThreadID] = entries[session.ThreadID].runtimeState.Running
	}
	if len(candidateOrder) == 0 {
		return nil
	}

	if len(previousOrder) == 0 {
		return buildInitialSessionWindowOrder(candidateOrder, activeByThreadID, limit)
	}

	prevActive := make([]string, 0, len(previousOrder))
	prevInactive := make([]string, 0, len(previousOrder))
	seen := map[string]bool{}
	for _, threadID := range previousOrder {
		if _, ok := candidateByThreadID[threadID]; !ok {
			continue
		}
		seen[threadID] = true
		if activeByThreadID[threadID] {
			prevActive = append(prevActive, threadID)
		} else {
			prevInactive = append(prevInactive, threadID)
		}
	}

	newActive := make([]string, 0)
	newInactive := make([]string, 0)
	for _, threadID := range candidateOrder {
		if seen[threadID] {
			continue
		}
		if activeByThreadID[threadID] {
			newActive = append(newActive, threadID)
		} else {
			newInactive = append(newInactive, threadID)
		}
	}

	activeOrder := append([]string{}, prevActive...)
	activeOrder = append(activeOrder, newActive...)

	inactiveOrder := make([]string, 0, len(prevInactive)+len(newInactive))
	for _, threadID := range previousOrder {
		if _, ok := candidateByThreadID[threadID]; !ok || activeByThreadID[threadID] {
			continue
		}
		if !slices.Contains(prevInactive, threadID) {
			continue
		}
		inactiveOrder = append(inactiveOrder, threadID)
	}
	for _, threadID := range candidateOrder {
		if _, ok := candidateByThreadID[threadID]; !ok || activeByThreadID[threadID] {
			continue
		}
		if seen[threadID] {
			continue
		}
		inactiveOrder = append(inactiveOrder, threadID)
	}

	order := append(activeOrder, inactiveOrder...)
	if len(order) > limit {
		order = order[:limit]
	}
	return order
}

func buildInitialSessionWindowOrder(candidateOrder []string, activeByThreadID map[string]bool, limit int) []string {
	active := make([]string, 0, len(candidateOrder))
	inactive := make([]string, 0, len(candidateOrder))
	for _, threadID := range candidateOrder {
		if activeByThreadID[threadID] {
			active = append(active, threadID)
		} else {
			inactive = append(inactive, threadID)
		}
	}
	order := append(active, inactive...)
	if len(order) > limit {
		order = order[:limit]
	}
	return order
}

func (a *App) tailSessionWindowEntries(
	w http.ResponseWriter,
	flusher http.Flusher,
	state sessionWindowStreamState,
) error {
	for _, threadID := range state.order {
		entry := state.entries[threadID]
		if entry == nil || entry.runtimeMachine == nil || entry.rolloutPath == "" {
			continue
		}
		updates, nextOffset, err := sessions.ReadStreamUpdatesFromOffset(entry.rolloutPath, threadID, entry.offset, entry.runtimeMachine)
		if err != nil {
			return err
		}
		entry.offset = nextOffset
		for _, update := range updates {
			if update.RuntimeState != nil {
				entry.runtimeState = *update.RuntimeState
				writeSessionWindowRuntimeStateEvent(w, flusher, *update.RuntimeState)
			}
			if update.AgentMessage != nil {
				copied := *update.AgentMessage
				entry.latestMessage = &copied
				writeSessionWindowAgentMessageEvent(w, flusher, copied)
			}
		}
	}
	return nil
}

func sessionWindowStatesEqual(left, right sessionWindowStreamState) bool {
	if !slices.Equal(left.order, right.order) {
		return false
	}
	if len(left.entries) != len(right.entries) {
		return false
	}
	for _, threadID := range left.order {
		leftEntry := left.entries[threadID]
		rightEntry := right.entries[threadID]
		if leftEntry == nil || rightEntry == nil {
			return false
		}
		if !sessionSnapshotsEqualForWindow(leftEntry.snapshot, rightEntry.snapshot) {
			return false
		}
		if !runtimeStatesEqualForWindow(leftEntry.runtimeState, rightEntry.runtimeState) {
			return false
		}
		if !agentMessagesEqualForWindow(leftEntry.latestMessage, rightEntry.latestMessage) {
			return false
		}
	}
	return true
}

func countActiveWindowEntries(entries map[string]*sessionWindowStreamEntry, order []string) int {
	count := 0
	for _, threadID := range order {
		if entry := entries[threadID]; entry != nil && entry.runtimeState.Running {
			count += 1
		}
	}
	return count
}

func diffThreadIDs(left, right []string) []string {
	var diff []string
	for _, threadID := range left {
		if !slices.Contains(right, threadID) {
			diff = append(diff, threadID)
		}
	}
	return diff
}

func sessionSnapshotsEqualForWindow(left, right sessions.SessionSnapshot) bool {
	return left.ThreadID == right.ThreadID &&
		left.Title == right.Title &&
		left.UpdatedAt.Equal(right.UpdatedAt) &&
		left.Model == right.Model &&
		left.ReasoningEffort == right.ReasoningEffort &&
		left.TokensUsedTotal == right.TokensUsedTotal &&
		left.ContextUsedTokens == right.ContextUsedTokens &&
		left.ContextWindow == right.ContextWindow &&
		left.ContextPressurePercent == right.ContextPressurePercent &&
		left.ContextCompactThresholdTokens == right.ContextCompactThresholdTokens &&
		left.ContextCompactThresholdPercent == right.ContextCompactThresholdPercent &&
		contextCompactionsEqual(left.ContextCompaction, right.ContextCompaction)
}

func contextCompactionsEqual(left, right *sessions.ContextCompactionSnapshot) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Trigger == right.Trigger &&
		left.StartedAt.Equal(right.StartedAt) &&
		left.UpdatedAt.Equal(right.UpdatedAt) &&
		left.TurnID == right.TurnID
}

func runtimeStatesEqualForWindow(left, right sessions.RuntimeState) bool {
	return left.Type == right.Type &&
		left.ThreadID == right.ThreadID &&
		left.TurnID == right.TurnID &&
		left.Running == right.Running &&
		left.Lifecycle == right.Lifecycle &&
		left.Phase == right.Phase &&
		left.UpdatedAt == right.UpdatedAt &&
		left.Sequence == right.Sequence
}

func agentMessagesEqualForWindow(left, right *sessions.AgentMessage) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Type == right.Type &&
		left.ThreadID == right.ThreadID &&
		left.EventID == right.EventID &&
		left.CreatedAt == right.CreatedAt &&
		left.Text == right.Text &&
		left.Truncated == right.Truncated
}

func buildSessionWindowEvent(state sessionWindowStreamState) sessionWindowEvent {
	items := make([]sessionWindowSessionData, 0, len(state.order))
	for _, threadID := range state.order {
		entry := state.entries[threadID]
		if entry == nil {
			continue
		}
		items = append(items, sessionWindowSessionData{
			ThreadID:                       entry.snapshot.ThreadID,
			Title:                          entry.snapshot.Title,
			Model:                          entry.snapshot.Model,
			ReasoningEffort:                entry.snapshot.ReasoningEffort,
			TokensUsedTotal:                entry.snapshot.TokensUsedTotal,
			ContextUsedTokens:              entry.snapshot.ContextUsedTokens,
			ContextWindow:                  entry.snapshot.ContextWindow,
			ContextPressurePercent:         entry.snapshot.ContextPressurePercent,
			ContextCompactThresholdTokens:  entry.snapshot.ContextCompactThresholdTokens,
			ContextCompactThresholdPercent: entry.snapshot.ContextCompactThresholdPercent,
			ContextCompaction:              entry.snapshot.ContextCompaction,
			LastActiveAgoMinutes:           entry.snapshot.LastActiveAgoMinutes,
			RuntimeState:                   entry.runtimeState,
			LatestAgentMessage:             entry.latestMessage,
		})
	}
	return sessionWindowEvent{
		Type:        "sessions_window",
		ObservedAt:  state.observedAt,
		Limit:       sessionWindowStreamLimit,
		ThreadOrder: append([]string(nil), state.order...),
		Sessions:    items,
	}
}

func writeSessionWindowEvent(w http.ResponseWriter, flusher http.Flusher, state sessionWindowStreamState) {
	writeSSE(w, flusher, "sessions_window", statusEventID("sessions_window", state.observedAt), buildSessionWindowEvent(state))
}

func writeSessionWindowRuntimeStateEvent(w http.ResponseWriter, flusher http.Flusher, runtimeState sessions.RuntimeState) {
	writeSSE(w, flusher, "session_runtime_state", "runtime-"+runtimeState.ThreadID+"-"+runtimeState.TurnID+"-"+strconv.FormatInt(runtimeState.Sequence, 10), sessionWindowRuntimeStateEvent{
		Type:         "session_runtime_state",
		RuntimeState: runtimeState,
	})
}

func writeSessionWindowAgentMessageEvent(w http.ResponseWriter, flusher http.Flusher, message sessions.AgentMessage) {
	writeSSE(w, flusher, "session_agent_message", "agent-"+message.ThreadID+"-"+message.EventID, sessionWindowAgentMessageEvent{
		Type:         "session_agent_message",
		AgentMessage: message,
	})
}

func writeWindowHeartbeatEvent(w http.ResponseWriter, flusher http.Flusher, now time.Time) {
	eventID := "heartbeat-" + now.Format("20060102150405.000000000")
	writeSSE(w, flusher, "heartbeat", eventID, heartbeatEvent{
		Type:      "heartbeat",
		CreatedAt: now.Format(time.RFC3339),
	})
}

func writeWindowStreamError(w http.ResponseWriter, flusher http.Flusher, now time.Time, message string) {
	eventID := "error-" + now.Format("20060102150405.000000000")
	writeSSE(w, flusher, "error", eventID, streamErrorEvent{
		Type:      "error",
		Message:   message,
		CreatedAt: now.Format(time.RFC3339),
	})
}

type sessionWindowRuntimeStateEvent struct {
	Type         string                `json:"type"`
	RuntimeState sessions.RuntimeState `json:"runtimeState"`
}

type sessionWindowAgentMessageEvent struct {
	Type         string                `json:"type"`
	AgentMessage sessions.AgentMessage `json:"agentMessage"`
}

func logSessionWindowStreamSummary(action string, fields map[string]any) {
	payload := map[string]any{
		"action": action,
	}
	for key, value := range fields {
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("watcher_session_window_stream_event {\"action\":\"%s\",\"marshalError\":%q}", action, err.Error())
		return
	}
	log.Printf("watcher_session_window_stream_event %s", data)
}
