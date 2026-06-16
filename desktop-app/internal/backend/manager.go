package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/processutil"
	rootconfig "openwatcher/internal/config"
)

type StartConfig struct {
	ConfigPath    string `json:"configPath"`
	Listen        string `json:"listen"`
	PublicBaseURL string `json:"publicBaseUrl"`
	PairingSlot   string `json:"pairingSlot,omitempty"`
}

type DesktopStatus struct {
	State                   string        `json:"state"`
	Message                 string        `json:"message"`
	ResolvedBinary          string        `json:"resolvedBinary"`
	BinarySource            string        `json:"binarySource"`
	FriendlyError           string        `json:"friendlyError,omitempty"`
	Running                 bool          `json:"running"`
	StartedAt               string        `json:"startedAt,omitempty"`
	RecentLogCount          int           `json:"recentLogCount"`
	HealthProbePath         string        `json:"healthProbePath"`
	ConfigPathLabel         string        `json:"configPathLabel,omitempty"`
	ConfiguredListen        string        `json:"configuredListen,omitempty"`
	ConfiguredPublicBaseURL string        `json:"configuredPublicBaseUrl,omitempty"`
	ConfiguredPairingSlot   string        `json:"configuredPairingSlot,omitempty"`
	LastHealth              *HealthStatus `json:"lastHealth,omitempty"`
}

type HealthStatus struct {
	OK       bool        `json:"ok"`
	Message  string      `json:"message"`
	HTTPCode int         `json:"httpCode,omitempty"`
	Endpoint string      `json:"endpoint,omitempty"`
	Build    BuildInfo   `json:"build"`
	Config   RuntimeInfo `json:"config"`
	Codex    CodexInfo   `json:"codex"`
	RawBody  string      `json:"rawBody,omitempty"`
}

type LogLine struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type BuildInfo struct {
	Version string `json:"version,omitempty"`
	Commit  string `json:"commit,omitempty"`
	BuiltAt string `json:"builtAt,omitempty"`
}

type RuntimeInfo struct {
	Listen        string `json:"listen,omitempty"`
	PublicBaseURL string `json:"publicBaseUrl,omitempty"`
	PairingSlot   string `json:"pairingSlot,omitempty"`
	Paired        bool   `json:"paired"`
	NoAuth        bool   `json:"noAuth"`
}

type CodexInfo struct {
	HomeDetected     bool `json:"homeDetected"`
	AuthDetected     bool `json:"authDetected"`
	SessionsDetected bool `json:"sessionsDetected"`
}

type Manager struct {
	locator  *BinaryLocator
	redactor *logging.Redactor

	mu         sync.Mutex
	cmd        *exec.Cmd
	logs       []LogLine
	startedAt  time.Time
	resolved   ResolvedBinary
	lastHealth *HealthStatus
	lastStart  StartConfig
	lastError  string
}

func NewManager(locator *BinaryLocator, redactor *logging.Redactor) *Manager {
	return &Manager{
		locator:  locator,
		redactor: redactor,
	}
}

func (m *Manager) StartBackend(ctx context.Context, cfg StartConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return nil
	}
	cfg = normalizeStartConfig(cfg)

	resolved, err := m.locator.Resolve()
	if err != nil {
		m.lastError = m.locator.FriendlyError()
		m.appendLocked("backend sidecar missing: " + m.lastError)
		return err
	}

	args := []string{"serve"}
	if cfg.ConfigPath != "" {
		args = append(args, "--config", cfg.ConfigPath)
	}
	if cfg.Listen != "" {
		args = append(args, "--listen", cfg.Listen)
	}
	if cfg.PublicBaseURL != "" {
		args = append(args, "--public-base-url", cfg.PublicBaseURL)
	}
	if strings.TrimSpace(cfg.PairingSlot) != "" {
		args = append(args, "--pairing-slot", cfg.PairingSlot)
	}

	cmd := exec.CommandContext(ctx, resolved.Path, args...)
	processutil.HideConsoleWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	m.cmd = cmd
	m.startedAt = time.Now()
	m.resolved = resolved
	m.lastStart = cfg
	m.lastError = ""
	go m.captureStream(stdout)
	go m.captureStream(stderr)
	go m.waitForExit(cmd)
	return nil
}

func (m *Manager) StopBackend(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	err := m.cmd.Process.Kill()
	m.cmd = nil
	m.lastError = ""
	m.appendLocked("backend sidecar stopped")
	return err
}

func (m *Manager) RestartBackend(ctx context.Context, cfg StartConfig) error {
	_ = m.StopBackend(ctx)
	return m.StartBackend(ctx, cfg)
}

func (m *Manager) CheckHealth(ctx context.Context) (HealthStatus, error) {
	return m.CheckHealthWithConfig(ctx, m.startConfig())
}

func (m *Manager) CheckHealthWithConfig(ctx context.Context, cfg StartConfig) (HealthStatus, error) {
	cfg = normalizeStartConfig(cfg)
	endpoint := deriveHealthEndpoint(cfg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return HealthStatus{Message: "健康检查构造失败"}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		status := HealthStatus{
			Endpoint: endpoint,
			Message:  "尚未连接到 sidecar /healthz。",
		}
		m.mu.Lock()
		m.lastHealth = &status
		m.mu.Unlock()
		return status, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if readErr != nil {
		return HealthStatus{Endpoint: endpoint, Message: "读取健康检查响应失败"}, readErr
	}

	type healthPayload struct {
		OK     bool        `json:"ok"`
		Build  BuildInfo   `json:"build"`
		Config RuntimeInfo `json:"config"`
		Codex  CodexInfo   `json:"codex"`
	}
	payload := healthPayload{}
	if err := json.Unmarshal(body, &payload); err != nil {
		status := HealthStatus{
			Endpoint: endpoint,
			HTTPCode: resp.StatusCode,
			OK:       resp.StatusCode >= 200 && resp.StatusCode < 300,
			Message:  "healthz 返回无法解析",
			RawBody:  m.redactor.RedactLine(string(body)),
		}
		m.mu.Lock()
		m.lastHealth = &status
		m.mu.Unlock()
		return status, err
	}

	status := HealthStatus{
		OK:       payload.OK && resp.StatusCode >= 200 && resp.StatusCode < 300,
		Message:  fmt.Sprintf("HTTP %d", resp.StatusCode),
		HTTPCode: resp.StatusCode,
		Endpoint: endpoint,
		Build:    payload.Build,
		Config:   payload.Config,
		Codex:    payload.Codex,
		RawBody:  m.redactor.RedactLine(string(body)),
	}
	m.mu.Lock()
	m.lastHealth = &status
	m.mu.Unlock()
	return status, nil
}

func (m *Manager) DesktopStatus() DesktopStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	resolved, err := m.locator.Resolve()
	if err == nil {
		m.resolved = resolved
	}

	status := DesktopStatus{
		State:                   "missing",
		Message:                 "OpenWatcher 本机服务尚未启动",
		ResolvedBinary:          trimRepoPath(m.resolved.Path),
		BinarySource:            m.resolved.Label,
		HealthProbePath:         deriveHealthEndpoint(m.lastStart),
		ConfigPathLabel:         trimHomePath(m.lastStart.ConfigPath),
		ConfiguredListen:        m.lastStart.Listen,
		ConfiguredPublicBaseURL: m.lastStart.PublicBaseURL,
		ConfiguredPairingSlot:   m.lastStart.PairingSlot,
		RecentLogCount:          len(m.logs),
		LastHealth:              m.lastHealth,
	}

	if err != nil {
		status.FriendlyError = m.locator.FriendlyError()
		status.Message = status.FriendlyError
	}
	if strings.TrimSpace(m.lastError) != "" {
		status.Message = m.lastError
		status.FriendlyError = m.lastError
		status.State = "error"
	}
	if m.cmd != nil && m.cmd.Process != nil {
		status.State = "running"
		status.Message = "OpenWatcher 本机服务已启动。"
		status.Running = true
		status.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	return status
}

func (m *Manager) GetBackendLogs(limit int) []LogLine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || len(m.logs) <= limit {
		return append([]LogLine(nil), m.logs...)
	}
	return append([]LogLine(nil), m.logs[len(m.logs)-limit:]...)
}

func (m *Manager) captureStream(stream interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		m.mu.Lock()
		m.appendLocked(scanner.Text())
		m.mu.Unlock()
	}
}

func (m *Manager) appendLocked(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	line := LogLine{
		At:      time.Now().Format(time.RFC3339),
		Message: m.redactor.RedactLine(message),
	}
	m.logs = append(m.logs, line)
	if len(m.logs) > 400 {
		m.logs = append([]LogLine(nil), m.logs[len(m.logs)-400:]...)
	}
}

func trimRepoPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	for _, marker := range []string{"/bundled/", "/bin/", "/build/"} {
		if idx := strings.Index(path, marker); idx >= 0 {
			return path[idx+1:]
		}
	}
	return path
}

func (m *Manager) waitForExit(cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != cmd {
		return
	}
	m.cmd = nil
	if err != nil {
		m.lastError = classifyProcessError(err.Error(), m.logs)
		m.appendLocked("backend sidecar exited: " + m.lastError)
	}
}

func (m *Manager) startConfig() StartConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastStart
}

func normalizeStartConfig(cfg StartConfig) StartConfig {
	if strings.TrimSpace(cfg.ConfigPath) == "" {
		cfg.ConfigPath = rootconfig.ResolvePathOrEmpty()
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		cfg.Listen = rootconfig.DefaultListen
	}
	cfg.PublicBaseURL = rootconfig.NormalizePublicBaseURL(cfg.PublicBaseURL)
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = rootconfig.DefaultPublicBaseURL(cfg.Listen)
	}
	cfg.PairingSlot = strings.TrimSpace(cfg.PairingSlot)
	return cfg
}

func deriveHealthEndpoint(cfg StartConfig) string {
	listen := strings.TrimSpace(cfg.Listen)
	if listen == "" {
		listen = rootconfig.DefaultListen
	}
	return rootconfig.DefaultPublicBaseURL(listen) + "/healthz"
}

func classifyProcessError(exitMessage string, logs []LogLine) string {
	candidates := []string{exitMessage}
	if len(logs) > 0 {
		candidates = append(candidates, logs[len(logs)-1].Message)
	}
	for _, candidate := range candidates {
		lower := strings.ToLower(candidate)
		switch {
		case strings.Contains(lower, "address already in use"):
			return "监听地址已被占用，请更换端口或停止占用进程。"
		case strings.Contains(lower, "codex access token missing"):
			return "Codex 当前未登录，缺少 access token。"
		case strings.Contains(lower, "permission denied"):
			return "启动 sidecar 时遇到权限问题，请检查二进制可执行权限和配置目录权限。"
		}
	}
	return strings.TrimSpace(exitMessage)
}

func trimHomePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	home, err := rootconfig.ResolveUserHome()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
