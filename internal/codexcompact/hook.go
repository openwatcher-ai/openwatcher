package codexcompact

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	rootconfig "openwatcher/internal/config"
)

const defaultStateSubdir = ".openwatcher/cache/codex-compact-state"

type HookInput struct {
	CWD           string `json:"cwd,omitempty"`
	HookEventName string `json:"hook_event_name,omitempty"`
	Model         string `json:"model,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	Transcript    string `json:"transcript_path,omitempty"`
	Trigger       string `json:"trigger,omitempty"`
	TurnID        string `json:"turn_id,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentType     string `json:"agent_type,omitempty"`
}

type State struct {
	SessionID string `json:"sessionId"`
	TurnID    string `json:"turnId,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	Model     string `json:"model,omitempty"`
	StartedAt string `json:"startedAt"`
	UpdatedAt string `json:"updatedAt"`
	Source    string `json:"source"`
}

func HandleHook(r io.Reader, now time.Time) error {
	payload, err := io.ReadAll(io.LimitReader(r, 1024*1024))
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return nil
	}

	var input HookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		return err
	}

	switch normalizeEventName(input.HookEventName) {
	case "precompact":
		return writeState(input, now)
	case "postcompact":
		return clearState(input)
	default:
		return nil
	}
}

func StateDir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("OPENWATCHER_COMPACT_STATE_DIR")); override != "" {
		return override, nil
	}
	home, err := rootconfig.ResolveUserHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultStateSubdir), nil
}

func StatePath(sessionID string) (string, error) {
	safeID := sanitizeSessionID(sessionID)
	if safeID == "" {
		return "", errors.New("session_id 为空")
	}
	stateDir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(stateDir, safeID+".json"), nil
}

func writeState(input HookInput, now time.Time) error {
	path, err := StatePath(input.SessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	timestamp := now.UTC().Format(time.RFC3339Nano)
	state := State{
		SessionID: strings.TrimSpace(input.SessionID),
		TurnID:    strings.TrimSpace(input.TurnID),
		Trigger:   normalizeTrigger(input.Trigger),
		Model:     strings.TrimSpace(input.Model),
		StartedAt: timestamp,
		UpdatedAt: timestamp,
		Source:    "codex-hook",
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(path, 0o600)
}

func clearState(input HookInput) error {
	path, err := StatePath(input.SessionID)
	if err != nil {
		return err
	}

	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	inputTurnID := strings.TrimSpace(input.TurnID)
	if inputTurnID != "" {
		var state State
		if err := json.Unmarshal(current, &state); err == nil && strings.TrimSpace(state.TurnID) != "" && strings.TrimSpace(state.TurnID) != inputTurnID {
			return nil
		}
	}
	return os.Remove(path)
}

func normalizeEventName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

func normalizeTrigger(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "auto", "manual":
		return normalized
	default:
		return strings.TrimSpace(value)
	}
}

func sanitizeSessionID(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), ".")
}
