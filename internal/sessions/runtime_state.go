package sessions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
)

type RuntimeLifecycle string

const (
	RuntimeLifecycleIdle      RuntimeLifecycle = "idle"
	RuntimeLifecycleRunning   RuntimeLifecycle = "running"
	RuntimeLifecycleCompleted RuntimeLifecycle = "completed"
	RuntimeLifecycleAborted   RuntimeLifecycle = "aborted"
)

type RuntimePhase string

const (
	RuntimePhaseUnknown         RuntimePhase = "unknown"
	RuntimePhaseReasoning       RuntimePhase = "reasoning"
	RuntimePhaseToolRunning     RuntimePhase = "tool_running"
	RuntimePhaseAgentCommentary RuntimePhase = "agent_commentary"
	RuntimePhaseAgentFinal      RuntimePhase = "agent_final"
)

type RuntimeState struct {
	Type      string           `json:"type"`
	ThreadID  string           `json:"threadId"`
	TurnID    string           `json:"turnId"`
	StartedAt string           `json:"startedAt,omitempty"`
	Running   bool             `json:"running"`
	Lifecycle RuntimeLifecycle `json:"lifecycle"`
	Phase     RuntimePhase     `json:"phase"`
	UpdatedAt string           `json:"updatedAt"`
	Sequence  int64            `json:"sequence"`
}

type StreamUpdate struct {
	RuntimeState *RuntimeState
	AgentMessage *AgentMessage
}

type RuntimeStateMachine struct {
	threadID      string
	sequence      int64
	state         RuntimeState
	openToolCalls map[string]bool
}

func NewRuntimeStateMachine(threadID string) *RuntimeStateMachine {
	return &RuntimeStateMachine{
		threadID: threadID,
		state: RuntimeState{
			Type:      "runtime_state",
			ThreadID:  threadID,
			Lifecycle: RuntimeLifecycleIdle,
			Phase:     RuntimePhaseUnknown,
		},
		openToolCalls: map[string]bool{},
	}
}

func (m *RuntimeStateMachine) State() RuntimeState {
	return m.state
}

func (m *RuntimeStateMachine) ApplyLine(line []byte, offset int64) (*RuntimeState, *AgentMessage) {
	event, ok := parseRolloutRuntimeLine(line)
	if !ok {
		return nil, nil
	}

	before := m.state
	message, hasMessage := buildAgentMessageFromRuntimeEvent(m.threadID, event, offset)

	switch event.Type {
	case "event_msg":
		m.applyEventMessage(event)
	case "response_item":
		m.applyResponseItem(event)
	}

	if !runtimeStatesEqual(before, m.state) {
		m.sequence++
		next := m.state
		next.Sequence = m.sequence
		m.state.Sequence = m.sequence
		return &next, messageIfPresent(message, hasMessage)
	}
	return nil, messageIfPresent(message, hasMessage)
}

func (m *RuntimeStateMachine) applyEventMessage(event rolloutRuntimeEvent) {
	switch event.Payload.Type {
	case "task_started":
		m.openToolCalls = map[string]bool{}
		m.state.Running = true
		m.state.Lifecycle = RuntimeLifecycleRunning
		m.state.Phase = RuntimePhaseUnknown
		if turnID := firstNonBlank(event.Payload.TurnID, event.Payload.TurnIDCamel, event.TurnID, event.TurnIDCamel); turnID != "" {
			m.state.TurnID = turnID
		}
		m.state.StartedAt = normalizeEventTimestamp(event.Timestamp)
		m.state.UpdatedAt = normalizeEventTimestamp(event.Timestamp)
	case "task_complete":
		if m.appliesToCurrentTurn(event) {
			m.openToolCalls = map[string]bool{}
			m.state.Running = false
			m.state.Lifecycle = RuntimeLifecycleCompleted
			m.state.UpdatedAt = normalizeEventTimestamp(event.Timestamp)
		}
	case "turn_aborted":
		if m.appliesToCurrentTurn(event) {
			m.openToolCalls = map[string]bool{}
			m.state.Running = false
			m.state.Lifecycle = RuntimeLifecycleAborted
			m.state.UpdatedAt = normalizeEventTimestamp(event.Timestamp)
		}
	case "agent_message":
		switch strings.TrimSpace(event.Payload.Phase) {
		case "commentary":
			m.setRunningPhase(RuntimePhaseAgentCommentary, event.Timestamp)
		case "final_answer":
			m.setRunningPhase(RuntimePhaseAgentFinal, event.Timestamp)
		}
	}
}

func (m *RuntimeStateMachine) applyResponseItem(event rolloutRuntimeEvent) {
	switch event.Payload.Type {
	case "reasoning":
		m.setRunningPhase(RuntimePhaseReasoning, event.Timestamp)
	case "function_call":
		if callID := strings.TrimSpace(event.Payload.CallID); callID != "" {
			m.openToolCalls[callID] = true
		}
		m.setRunningPhase(RuntimePhaseToolRunning, event.Timestamp)
	case "function_call_output":
		if callID := strings.TrimSpace(event.Payload.CallID); callID != "" {
			delete(m.openToolCalls, callID)
		}
		if len(m.openToolCalls) > 0 {
			m.setRunningPhase(RuntimePhaseToolRunning, event.Timestamp)
		}
	}
}

func (m *RuntimeStateMachine) setRunningPhase(phase RuntimePhase, timestamp string) {
	if m.state.Lifecycle != RuntimeLifecycleRunning {
		return
	}
	if len(m.openToolCalls) > 0 {
		phase = RuntimePhaseToolRunning
	}
	m.state.Phase = phase
	m.state.UpdatedAt = normalizeEventTimestamp(timestamp)
}

func (m *RuntimeStateMachine) appliesToCurrentTurn(event rolloutRuntimeEvent) bool {
	turnID := firstNonBlank(event.Payload.TurnID, event.Payload.TurnIDCamel, event.TurnID, event.TurnIDCamel)
	return turnID == "" || m.state.TurnID == "" || turnID == m.state.TurnID
}

func LatestRuntimeState(path, threadID string) (RuntimeState, int64, error) {
	state, offset, _, err := RecoverRuntimeState(path, threadID)
	return state, offset, err
}

func RecoverRuntimeState(path, threadID string) (RuntimeState, int64, *RuntimeStateMachine, error) {
	file, err := os.Open(path)
	if err != nil {
		return RuntimeState{}, 0, nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return RuntimeState{}, 0, nil, err
	}
	endOffset := info.Size()
	machine := NewRuntimeStateMachine(threadID)
	if endOffset <= 0 {
		return machine.State(), 0, machine, nil
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return RuntimeState{}, 0, nil, err
	}

	reader := bufio.NewReader(file)
	currentOffset := int64(0)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			lineOffset := currentOffset
			currentOffset += int64(len(line))
			_, _ = machine.ApplyLine(line, lineOffset)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return RuntimeState{}, currentOffset, nil, err
		}
	}
	return machine.State(), endOffset, machine, nil
}

func ReadStreamUpdatesFromOffset(path, threadID string, offset int64, machine *RuntimeStateMachine) ([]StreamUpdate, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}
	if info.Size() < offset {
		offset = info.Size()
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	reader := bufio.NewReader(file)
	currentOffset := offset
	var updates []StreamUpdate
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if err == io.EOF && !bytes.HasSuffix(line, []byte{'\n'}) {
				break
			}
			lineOffset := currentOffset
			currentOffset += int64(len(line))
			state, message := machine.ApplyLine(line, lineOffset)
			if state != nil || message != nil {
				updates = append(updates, StreamUpdate{RuntimeState: state, AgentMessage: message})
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, currentOffset, err
		}
	}
	return updates, currentOffset, nil
}

type rolloutRuntimeEvent struct {
	Timestamp   string                `json:"timestamp"`
	Type        string                `json:"type"`
	TurnID      string                `json:"turn_id"`
	TurnIDCamel string                `json:"turnId"`
	Payload     rolloutRuntimePayload `json:"payload"`
}

type rolloutRuntimePayload struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Phase       string `json:"phase"`
	TurnID      string `json:"turn_id"`
	TurnIDCamel string `json:"turnId"`
	CallID      string `json:"call_id"`
}

func parseRolloutRuntimeLine(line []byte) (rolloutRuntimeEvent, bool) {
	var event rolloutRuntimeEvent
	if err := json.Unmarshal(bytes.TrimSpace(line), &event); err != nil {
		return rolloutRuntimeEvent{}, false
	}
	if event.Type != "event_msg" && event.Type != "response_item" {
		return rolloutRuntimeEvent{}, false
	}
	return event, true
}

func buildAgentMessageFromRuntimeEvent(threadID string, event rolloutRuntimeEvent, offset int64) (AgentMessage, bool) {
	if event.Type != "event_msg" || event.Payload.Type != "agent_message" {
		return AgentMessage{}, false
	}
	text, truncated := trimAgentMessageText(event.Payload.Message)
	if text == "" {
		return AgentMessage{}, false
	}
	return AgentMessage{
		Type:      "agent_message",
		ThreadID:  threadID,
		EventID:   strconv.FormatInt(offset, 10),
		CreatedAt: normalizeEventTimestamp(event.Timestamp),
		Text:      text,
		Truncated: truncated,
	}, true
}

func messageIfPresent(message AgentMessage, ok bool) *AgentMessage {
	if !ok {
		return nil
	}
	return &message
}

func runtimeStatesEqual(left, right RuntimeState) bool {
	return left.TurnID == right.TurnID &&
		left.StartedAt == right.StartedAt &&
		left.Running == right.Running &&
		left.Lifecycle == right.Lifecycle &&
		left.Phase == right.Phase &&
		left.UpdatedAt == right.UpdatedAt
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
