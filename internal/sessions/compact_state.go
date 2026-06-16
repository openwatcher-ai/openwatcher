package sessions

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"openwatcher/internal/codexcompact"
)

const contextCompactionStaleAfter = 30 * time.Minute

type contextCompactionState struct {
	state     codexcompact.State
	path      string
	startedAt time.Time
	updatedAt time.Time
}

func loadContextCompactionStates(now time.Time) map[string]contextCompactionState {
	stateDir, err := codexcompact.StateDir()
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(stateDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return nil
	}

	states := make(map[string]contextCompactionState)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(stateDir, entry.Name())
		loaded, ok := readContextCompactionState(path, now)
		if !ok {
			continue
		}
		states[loaded.state.SessionID] = loaded
	}
	return states
}

func readContextCompactionState(path string, now time.Time) (contextCompactionState, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contextCompactionState{}, false
	}
	var state codexcompact.State
	if err := json.Unmarshal(data, &state); err != nil {
		return contextCompactionState{}, false
	}
	if strings.TrimSpace(state.SessionID) == "" {
		return contextCompactionState{}, false
	}
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(state.StartedAt))
	if err != nil {
		return contextCompactionState{}, false
	}
	updatedAt := startedAt
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(state.UpdatedAt)); err == nil {
		updatedAt = parsed
	}
	if isContextCompactionStale(startedAt, now) {
		logContextCompactionStateCleared(state.SessionID, "stale", path)
		_ = os.Remove(path)
		return contextCompactionState{}, false
	}
	return contextCompactionState{
		state:     state,
		path:      path,
		startedAt: startedAt,
		updatedAt: updatedAt,
	}, true
}

func contextCompactionForRow(row threadRow, compaction contextCompactionState, now time.Time) *ContextCompactionSnapshot {
	if strings.TrimSpace(compaction.state.SessionID) == "" {
		return nil
	}
	if isContextCompactionStale(compaction.startedAt, now) || rolloutHasCompactFailure(row.RolloutPath, compaction) {
		if compaction.path != "" {
			logContextCompactionStateCleared(row.ThreadID, "failed_or_stale", compaction.path)
			_ = os.Remove(compaction.path)
		}
		return nil
	}
	return &ContextCompactionSnapshot{
		Trigger:   strings.TrimSpace(compaction.state.Trigger),
		StartedAt: compaction.startedAt,
		UpdatedAt: compaction.updatedAt,
		TurnID:    strings.TrimSpace(compaction.state.TurnID),
	}
}

func logContextCompactionStateCleared(threadID string, reason string, path string) {
	if strings.TrimSpace(threadID) == "" {
		return
	}
	log.Printf("context_compaction_state_file_cleared threadId=%s reason=%s file=%s", threadID, reason, filepath.Base(path))
}

func isContextCompactionStale(startedAt time.Time, now time.Time) bool {
	if startedAt.IsZero() || now.IsZero() {
		return false
	}
	return now.Sub(startedAt) > contextCompactionStaleAfter
}

func rolloutHasCompactFailure(path string, compaction contextCompactionState) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "Error running remote compact task") {
			continue
		}
		if eventAfterCompactionStarted(line, compaction.startedAt) {
			return true
		}
	}
	return false
}

func eventAfterCompactionStarted(line string, startedAt time.Time) bool {
	var event struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return true
	}
	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(event.Timestamp))
	if err != nil {
		return true
	}
	return !timestamp.Before(startedAt)
}
