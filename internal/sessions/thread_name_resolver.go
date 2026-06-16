package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	threadNameResolverListLimitMultiplier = 8
	threadNameResolverMinListLimit        = 32
)

var allThreadSourceKinds = []string{
	"cli",
	"vscode",
	"exec",
	"appServer",
	"subAgent",
	"subAgentReview",
	"subAgentCompact",
	"subAgentThreadSpawn",
	"subAgentOther",
	"unknown",
}

type threadNameResolverFunc func(ctx context.Context, codexHome string, threadIDs []string) (map[string]string, error)

type appServerRPCClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	enc    *json.Encoder
	dec    *json.Decoder
	stderr strings.Builder
}

type jsonRPCMessage struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	ClientInfo   clientInfo              `json:"clientInfo"`
	Capabilities *initializeCapabilities `json:"capabilities,omitempty"`
}

type clientInfo struct {
	Name    string  `json:"name"`
	Title   *string `json:"title,omitempty"`
	Version string  `json:"version"`
}

type initializeCapabilities struct {
	ExperimentalAPI        bool     `json:"experimentalApi"`
	RequestAttestation     bool     `json:"requestAttestation"`
	OptOutNotificationList []string `json:"optOutNotificationMethods,omitempty"`
}

type threadListParams struct {
	Limit          int      `json:"limit"`
	SortKey        string   `json:"sortKey"`
	SortDirection  string   `json:"sortDirection"`
	SourceKinds    []string `json:"sourceKinds"`
	Archived       bool     `json:"archived"`
	UseStateDBOnly bool     `json:"useStateDbOnly"`
}

type threadListResponse struct {
	Data []threadSummary `json:"data"`
}

type threadReadParams struct {
	ThreadID     string `json:"threadId"`
	IncludeTurns bool   `json:"includeTurns"`
}

type threadReadResponse struct {
	Thread threadSummary `json:"thread"`
}

type threadSummary struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

func resolveThreadNamesFromAppServer(ctx context.Context, codexHome string, threadIDs []string) (map[string]string, error) {
	if len(threadIDs) == 0 {
		return nil, nil
	}

	client, err := startAppServerRPCClient(ctx, codexHome)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.initialize(); err != nil {
		return nil, err
	}

	names, err := client.listThreadNames(max(len(threadIDs)*threadNameResolverListLimitMultiplier, threadNameResolverMinListLimit))
	if err != nil {
		return nil, err
	}

	for _, threadID := range threadIDs {
		if _, ok := names[threadID]; ok {
			continue
		}
		name, err := client.readThreadName(threadID)
		if err != nil || name == "" {
			continue
		}
		names[threadID] = name
	}

	filtered := make(map[string]string, len(threadIDs))
	for _, threadID := range threadIDs {
		if name := normalizeThreadName(names[threadID]); name != "" {
			filtered[threadID] = name
		}
	}
	return filtered, nil
}

func startAppServerRPCClient(ctx context.Context, codexHome string) (*appServerRPCClient, error) {
	cmd := exec.CommandContext(ctx, "codex", "app-server", "--stdio")
	cmd.Env = append(os.Environ(), "CODEX_HOME="+codexHome)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open app-server stdout: %w", err)
	}

	client := &appServerRPCClient{
		cmd:   cmd,
		stdin: stdin,
		enc:   json.NewEncoder(stdin),
		dec:   json.NewDecoder(stdout),
	}
	cmd.Stderr = &client.stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start codex app-server: %w%s", err, client.stderrSuffix())
	}
	return client, nil
}

func (c *appServerRPCClient) Close() {
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil {
		_ = c.cmd.Wait()
	}
}

func (c *appServerRPCClient) initialize() error {
	return c.call("initialize", initializeParams{
		ClientInfo: clientInfo{
			Name:    "openwatcher",
			Version: "0.0.0",
		},
		Capabilities: &initializeCapabilities{
			ExperimentalAPI:        true,
			RequestAttestation:     false,
			OptOutNotificationList: []string{"remoteControl/status/changed"},
		},
	}, nil)
}

func (c *appServerRPCClient) listThreadNames(limit int) (map[string]string, error) {
	var response threadListResponse
	if err := c.call("thread/list", threadListParams{
		Limit:          limit,
		SortKey:        "updated_at",
		SortDirection:  "desc",
		SourceKinds:    slices.Clone(allThreadSourceKinds),
		Archived:       false,
		UseStateDBOnly: false,
	}, &response); err != nil {
		return nil, err
	}

	names := make(map[string]string, len(response.Data))
	for _, thread := range response.Data {
		if thread.Name == nil {
			continue
		}
		if name := normalizeThreadName(*thread.Name); name != "" {
			names[thread.ID] = name
		}
	}
	return names, nil
}

func (c *appServerRPCClient) readThreadName(threadID string) (string, error) {
	var response threadReadResponse
	if err := c.call("thread/read", threadReadParams{
		ThreadID:     threadID,
		IncludeTurns: false,
	}, &response); err != nil {
		return "", err
	}
	if response.Thread.Name == nil {
		return "", nil
	}
	return normalizeThreadName(*response.Thread.Name), nil
}

func (c *appServerRPCClient) call(method string, params any, result any) error {
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      method,
		"method":  method,
		"params":  params,
	}
	if err := c.enc.Encode(request); err != nil {
		return fmt.Errorf("write %s request: %w%s", method, err, c.stderrSuffix())
	}

	for {
		var message jsonRPCMessage
		if err := c.dec.Decode(&message); err != nil {
			return fmt.Errorf("read %s response: %w%s", method, err, c.stderrSuffix())
		}
		if len(message.ID) == 0 {
			continue
		}

		var responseID string
		if err := json.Unmarshal(message.ID, &responseID); err != nil || responseID != method {
			continue
		}
		if message.Error != nil {
			return fmt.Errorf("%s failed: %s%s", method, message.Error.Message, c.stderrSuffix())
		}
		if result == nil || len(message.Result) == 0 {
			return nil
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("decode %s result: %w", method, err)
		}
		return nil
	}
}

func (c *appServerRPCClient) stderrSuffix() string {
	text := normalizeThreadName(c.stderr.String())
	if text == "" {
		return ""
	}
	return ": " + text
}
