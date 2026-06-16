package codexcompact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleHookWritesAndClearsCompactState(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("OPENWATCHER_COMPACT_STATE_DIR", stateDir)
	now := time.Date(2026, 6, 13, 15, 0, 0, 0, time.UTC)

	pre := `{"hook_event_name":"PreCompact","session_id":"019ec0f6-fb69-7882-a7dc-04b3c355739f","turn_id":"turn-1","trigger":"auto","model":"gpt-5"}`
	if err := HandleHook(strings.NewReader(pre), now); err != nil {
		t.Fatalf("HandleHook PreCompact err = %v", err)
	}

	path := filepath.Join(stateDir, "019ec0f6-fb69-7882-a7dc-04b3c355739f.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if state.SessionID != "019ec0f6-fb69-7882-a7dc-04b3c355739f" || state.TurnID != "turn-1" || state.Trigger != "auto" {
		t.Fatalf("state = %+v", state)
	}

	postOtherTurn := `{"hook_event_name":"PostCompact","session_id":"019ec0f6-fb69-7882-a7dc-04b3c355739f","turn_id":"turn-2"}`
	if err := HandleHook(strings.NewReader(postOtherTurn), now); err != nil {
		t.Fatalf("HandleHook PostCompact other turn err = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state should remain for mismatched turn: %v", err)
	}

	post := `{"hook_event_name":"PostCompact","session_id":"019ec0f6-fb69-7882-a7dc-04b3c355739f","turn_id":"turn-1"}`
	if err := HandleHook(strings.NewReader(post), now); err != nil {
		t.Fatalf("HandleHook PostCompact err = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state should be removed, err=%v", err)
	}
}

func TestStatePathSanitizesSessionID(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("OPENWATCHER_COMPACT_STATE_DIR", stateDir)

	path, err := StatePath("../session/id")
	if err != nil {
		t.Fatalf("StatePath err = %v", err)
	}
	if filepath.Dir(path) != stateDir {
		t.Fatalf("StatePath escaped state dir: %s", path)
	}
	if filepath.Base(path) != "_session_id.json" {
		t.Fatalf("StatePath base = %s", filepath.Base(path))
	}
}
