package sessions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// 详情页正文支持滚动，接口层保留更长消息，避免 2k+ 的最终答复被提前截断。
	agentMessageMaxRunes      = 4096
	agentMessageReadChunkSize = 256 * 1024
)

type AgentMessage struct {
	Type      string `json:"type"`
	ThreadID  string `json:"threadId"`
	EventID   string `json:"eventId"`
	CreatedAt string `json:"createdAt"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

type rolloutAgentMessageEvent struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Payload   struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"payload"`
}

func LatestAgentMessage(path, threadID string) (AgentMessage, int64, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return AgentMessage{}, 0, false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return AgentMessage{}, 0, false, err
	}
	endOffset := info.Size()
	if endOffset <= 0 {
		return AgentMessage{}, 0, false, nil
	}

	position := endOffset
	var carry []byte
	for position > 0 {
		readSize := position
		if readSize > agentMessageReadChunkSize {
			readSize = agentMessageReadChunkSize
		}
		position -= readSize
		if _, err := file.Seek(position, io.SeekStart); err != nil {
			return AgentMessage{}, 0, false, err
		}

		chunk := make([]byte, readSize)
		if _, err := io.ReadFull(file, chunk); err != nil {
			return AgentMessage{}, 0, false, err
		}

		data := append(chunk, carry...)
		baseOffset := position
		if position > 0 {
			newlineIndex := bytes.IndexByte(data, '\n')
			if newlineIndex < 0 {
				carry = data
				continue
			}
			carry = append(carry[:0], data[:newlineIndex+1]...)
			data = data[newlineIndex+1:]
			baseOffset += int64(newlineIndex + 1)
		} else {
			carry = nil
		}

		lines := splitLinesWithOffsets(data, baseOffset)
		for index := len(lines) - 1; index >= 0; index-- {
			message, ok := parseAgentMessageLine(threadID, lines[index].line, lines[index].offset)
			if ok {
				return message, endOffset, true, nil
			}
		}
	}
	return AgentMessage{}, endOffset, false, nil
}

func ReadAgentMessagesFromOffset(path, threadID string, offset int64) ([]AgentMessage, int64, error) {
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
	var messages []AgentMessage
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if err == io.EOF && !bytes.HasSuffix(line, []byte{'\n'}) {
				break
			}
			lineOffset := currentOffset
			currentOffset += int64(len(line))
			if message, ok := parseAgentMessageLine(threadID, line, lineOffset); ok {
				messages = append(messages, message)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, currentOffset, err
		}
	}
	return messages, currentOffset, nil
}

type lineWithOffset struct {
	line   []byte
	offset int64
}

func splitLinesWithOffsets(data []byte, baseOffset int64) []lineWithOffset {
	parts := bytes.SplitAfter(data, []byte{'\n'})
	lines := make([]lineWithOffset, 0, len(parts))
	currentOffset := baseOffset
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		lines = append(lines, lineWithOffset{line: part, offset: currentOffset})
		currentOffset += int64(len(part))
	}
	return lines
}

func parseAgentMessageLine(threadID string, line []byte, offset int64) (AgentMessage, bool) {
	if !bytes.Contains(line, []byte(`"agent_message"`)) {
		return AgentMessage{}, false
	}

	var event rolloutAgentMessageEvent
	if err := json.Unmarshal(bytes.TrimSpace(line), &event); err != nil {
		return AgentMessage{}, false
	}
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

func trimAgentMessageText(input string) (string, bool) {
	text := strings.TrimSpace(input)
	if text == "" {
		return "", false
	}

	runes := []rune(text)
	if len(runes) <= agentMessageMaxRunes {
		return text, false
	}
	return string(runes[:agentMessageMaxRunes]), true
}

func normalizeEventTimestamp(input string) string {
	timestamp := strings.TrimSpace(input)
	if timestamp == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return timestamp
	}
	return parsed.Format(time.RFC3339Nano)
}
