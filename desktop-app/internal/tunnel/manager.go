package tunnel

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/processutil"
	desktopruntime "openwatcher/desktop-app/internal/runtime"
	"openwatcher/desktop-app/internal/settings"
)

type Manager struct {
	locator  *BinaryLocator
	runtime  *desktopruntime.Manager
	store    *Store
	client   *Client
	redactor *logging.Redactor
	label    string

	mu                  sync.Mutex
	cmd                 *exec.Cmd
	logs                []LogLine
	startedAt           time.Time
	resolved            ResolvedBinary
	runningTunnelID     string
	runningOriginURL    string
	lastError           string
	lastRedeemErrorCode string
	tokenExpired        bool
	sharedNotice        string
	lastHealth          *HealthCheck
}

func NewManager(appRoot string, runtimeManager *desktopruntime.Manager, redactor *logging.Redactor) *Manager {
	return NewNamedManager(appRoot, runtimeManager, redactor, "managed-tunnel", "托管隧道")
}

func NewNamedManager(
	appRoot string,
	runtimeManager *desktopruntime.Manager,
	redactor *logging.Redactor,
	storeRoot string,
	label string,
) *Manager {
	configDir, err := settings.ConfigDir()
	if err != nil || strings.TrimSpace(configDir) == "" {
		configDir = settings.AppRoot()
	}
	trimmedLabel := strings.TrimSpace(label)
	if trimmedLabel == "" {
		trimmedLabel = "托管隧道"
	}
	return &Manager{
		locator:  NewBinaryLocator(appRoot),
		runtime:  runtimeManager,
		store:    NewNamedStore(configDir, storeRoot),
		client:   NewClient(),
		redactor: redactor,
		label:    trimmedLabel,
	}
}

func (m *Manager) EnsureIdentity() (Identity, error) {
	return m.store.EnsureIdentity()
}

func (m *Manager) Redeem(ctx context.Context, code string, desktopVersion string) (Status, error) {
	identity, err := m.store.EnsureIdentity()
	if err != nil {
		return m.Status(), err
	}
	response, err := m.client.Redeem(ctx, code, identity, desktopVersion)
	if err != nil {
		m.mu.Lock()
		m.lastError = err.Error()
		if redeemErr, ok := err.(*RedeemError); ok {
			m.lastRedeemErrorCode = redeemErr.Code
		} else {
			m.lastRedeemErrorCode = "network_error"
		}
		m.mu.Unlock()
		return m.Status(), err
	}
	if err := m.store.SaveBinding(Binding{
		PublicBaseURL: response.PublicBaseURL,
		TunnelID:      response.TunnelID,
		TokenVersion:  response.TokenVersion,
		RedeemedAt:    response.IssuedAt,
	}, response); err != nil {
		return m.Status(), err
	}

	m.mu.Lock()
	_ = m.stopLocked("managed tunnel stopped after new binding")
	m.lastError = "配置码兑换成功，已保存托管隧道绑定。"
	m.lastRedeemErrorCode = ""
	m.tokenExpired = false
	m.lastHealth = nil
	m.mu.Unlock()
	return m.Status(), nil
}

func (m *Manager) Start(ctx context.Context, originURL string) error {
	runtimeEnsureWarning := ""
	if m.runtime != nil {
		if err := m.runtime.EnsureTunnel(ctx); err != nil {
			if resolved, resolveErr := m.locator.Resolve(); resolveErr == nil {
				runtimeEnsureWarning = "运行时资源检查失败，继续使用已存在的" + m.label + "组件：" + err.Error()
				m.mu.Lock()
				m.resolved = resolved
				m.mu.Unlock()
			} else {
				m.mu.Lock()
				m.lastError = err.Error()
				m.appendLocked("启动" + m.label + "失败：" + m.lastError)
				m.mu.Unlock()
				return err
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if runtimeEnsureWarning != "" {
		m.appendLocked(runtimeEnsureWarning)
	}

	binding, token, err := m.store.LoadBinding()
	if err != nil {
		if errors.Is(err, ErrBindingNotFound) {
			m.lastError = "请先输入配置码并完成兑换，再启动" + m.label + "。"
			m.appendLocked("启动" + m.label + "失败：" + m.lastError)
			return err
		}
		m.lastError = "读取托管隧道配置失败。"
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return err
	}
	originURL = strings.TrimRight(strings.TrimSpace(originURL), "/")
	if originURL == "" {
		m.lastError = "缺少" + m.label + "本地转发地址。"
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return fmt.Errorf("managed tunnel origin url is empty")
	}
	if m.cmd != nil && m.cmd.Process != nil {
		if m.runningTunnelID == binding.TunnelID && m.runningOriginURL == originURL {
			return nil
		}
		_ = m.stopLocked("managed tunnel stopped before switching binding")
	}
	if err := m.store.WriteRunnerConfig(binding, originURL); err != nil {
		if _, statErr := os.Stat(m.store.CredentialsPath()); statErr == nil {
			m.lastError = "写入" + m.label + "运行时配置失败。"
			m.appendLocked("启动" + m.label + "失败：" + m.lastError)
			return err
		}
	}

	resolved, err := m.locator.Resolve()
	if err != nil {
		m.lastError = m.locator.FriendlyError()
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return err
	}

	m.sharedNotice = otherCloudflaredNotice()
	m.appendLocked(m.label + "启动请求。")
	args, err := cloudflaredRunArgs(m.store, binding, token, originURL)
	if err != nil {
		m.lastError = "缺少可用的托管隧道凭据。"
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return err
	}
	cmd := exec.CommandContext(ctx, resolved.Path, args...)
	processutil.HideConsoleWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		m.lastError = "读取 cloudflared stdout 失败。"
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		m.lastError = "读取 cloudflared stderr 失败。"
		m.appendLocked("启动" + m.label + "失败：" + m.lastError)
		return err
	}
	if err := cmd.Start(); err != nil {
		m.lastError = "启动" + m.label + "失败。"
		m.appendLocked(m.lastError)
		return err
	}

	m.cmd = cmd
	m.startedAt = time.Now()
	m.resolved = resolved
	m.runningTunnelID = binding.TunnelID
	m.runningOriginURL = originURL
	m.lastError = ""
	m.tokenExpired = false
	m.appendLocked(m.label + "进程已启动。")
	go m.captureStream(stdout)
	go m.captureStream(stderr)
	go m.waitForExit(cmd)
	return nil
}

func (m *Manager) Stop(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked("managed tunnel stopped")
}

func (m *Manager) stopLocked(message string) error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	err := m.cmd.Process.Kill()
	m.cmd = nil
	m.runningTunnelID = ""
	m.runningOriginURL = ""
	m.appendLocked(message)
	return err
}

func (m *Manager) RecordHealth(status backend.HealthStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastHealth = &HealthCheck{
		OK:        status.OK,
		Message:   status.Message,
		Endpoint:  status.Endpoint,
		CheckedAt: time.Now().Format(time.RFC3339),
	}
	if status.OK {
		m.appendLocked(m.label + "健康检查通过：" + firstNonBlank(status.Message, "OK"))
		return
	}
	m.appendLocked(m.label + "健康检查失败：" + firstNonBlank(status.Message, "未知错误"))
}

func (m *Manager) GetLogs(limit int) []LogLine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || len(m.logs) <= limit {
		return append([]LogLine(nil), m.logs...)
	}
	return append([]LogLine(nil), m.logs[len(m.logs)-limit:]...)
}

func (m *Manager) ClearLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = nil
	m.lastHealth = nil
}

func (m *Manager) PublicHealth(ctx context.Context) (backend.HealthStatus, error) {
	binding, _, err := m.store.LoadBinding()
	if err != nil {
		status := backend.HealthStatus{
			OK:      false,
			Message: "请先输入配置码并完成兑换，再测试" + m.label + "。",
		}
		m.RecordHealth(status)
		return status, err
	}

	m.mu.Lock()
	running := m.cmd != nil && m.cmd.Process != nil
	expired := m.tokenExpired
	m.mu.Unlock()
	if !running {
		status := backend.HealthStatus{
			OK:       false,
			Endpoint: strings.TrimRight(binding.PublicBaseURL, "/") + "/healthz",
			Message:  m.notRunningMessage(),
		}
		if expired {
			status.Message = TokenExpiredUserMessage
		}
		m.RecordHealth(status)
		return status, nil
	}

	status, err := backend.ProbePublicHealth(ctx, binding.PublicBaseURL, m.label+"公网地址")
	if expired && !status.OK {
		status.Message = TokenExpiredUserMessage
	}
	m.RecordHealth(status)
	return status, err
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	binding, token, err := m.store.LoadBinding()
	if err != nil && !errors.Is(err, ErrBindingNotFound) {
		m.lastError = "读取托管隧道配置失败。"
	}

	resolved, resolveErr := m.locator.Resolve()
	if resolveErr == nil {
		m.resolved = resolved
	}

	status := Status{
		State:               "unconfigured",
		Message:             "尚未兑换 OpenWatcher 托管隧道配置码。",
		Configured:          false,
		Running:             m.cmd != nil && m.cmd.Process != nil,
		TokenExpired:        m.tokenExpired,
		ResolvedBinary:      trimRepoPath(m.resolved.Path),
		BinarySource:        m.resolved.Label,
		LastRedeemErrorCode: m.lastRedeemErrorCode,
		SharedProcessNotice: m.sharedNotice,
		RecentLogCount:      len(m.logs),
		LastHealth:          m.lastHealth,
	}

	if err == nil {
		status.Configured = true
		status.PublicBaseURL = binding.PublicBaseURL
		status.TunnelID = binding.TunnelID
		status.TokenVersion = binding.TokenVersion
		status.RedeemedAt = binding.RedeemedAt
		status.TokenFingerprint = fingerprintToken(token)
		status.HealthProbePath = strings.TrimRight(binding.PublicBaseURL, "/") + "/healthz"
		status.State = "ready"
		status.Message = "已保存" + m.label + "配置，等待启动。"
	}
	if status.Running {
		status.State = "running"
		status.Message = m.label + "正在运行。"
		status.StartedAt = m.startedAt.Format(time.RFC3339)
	}
	if status.Configured && m.lastHealth != nil && !m.lastHealth.OK {
		status.State = "error"
		status.Message = firstNonBlank(m.lastHealth.Message, m.label+"公网健康检查失败。")
	}
	if strings.TrimSpace(m.lastError) != "" {
		status.Message = m.lastError
		if status.Configured {
			status.State = "error"
		}
	}
	if m.tokenExpired {
		status.State = "expired"
		status.Message = TokenExpiredUserMessage
	}
	if resolveErr != nil && !status.Running {
		status.Message = m.locator.FriendlyError()
		if status.Configured {
			status.State = "error"
		}
	}
	return status
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
	trimmed := strings.TrimSpace(m.redactor.RedactLine(message))
	if trimmed == "" {
		return
	}
	m.logs = append(m.logs, LogLine{
		At:      time.Now().Format(time.RFC3339),
		Message: trimmed,
	})
	if len(m.logs) > 300 {
		m.logs = append([]LogLine(nil), m.logs[len(m.logs)-300:]...)
	}
}

func (m *Manager) notRunningMessage() string {
	if strings.Contains(m.label, "开发") {
		return m.label + "进程尚未启动，请先启动开发环境，或重新打开开发隧道开关。"
	}
	return m.label + "进程尚未启动，请先启动本机服务。"
}

func (m *Manager) waitForExit(cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != cmd {
		return
	}
	m.cmd = nil
	m.runningTunnelID = ""
	m.runningOriginURL = ""
	if err == nil {
		m.lastError = "托管隧道已停止。"
		return
	}

	message, expired := classifyTunnelError(err.Error(), m.logs)
	m.tokenExpired = expired
	m.lastError = message
	m.appendLocked("managed tunnel exited: " + message)
}

func fingerprintToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])[:8]
}

func classifyTunnelError(exitMessage string, logs []LogLine) (string, bool) {
	candidates := []string{exitMessage}
	for _, line := range logs {
		candidates = append(candidates, line.Message)
	}
	for _, candidate := range candidates {
		lower := strings.ToLower(candidate)
		switch {
		case strings.Contains(lower, "provided tunnel token is not valid"),
			strings.Contains(lower, "token is not valid"),
			strings.Contains(lower, "invalid token"),
			strings.Contains(lower, "unauthorized"):
			return TokenExpiredUserMessage, true
		case strings.Contains(lower, "failed to read"),
			strings.Contains(lower, "no such file"),
			strings.Contains(lower, "permission denied"):
			return "读取托管隧道凭据失败，请检查 Desktop 配置目录权限。", false
		}
	}
	return "托管隧道进程异常退出，请检查网络或稍后重试。", false
}

func otherCloudflaredNotice() string {
	output, err := exec.Command("pgrep", "-af", "cloudflared").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	if count <= 0 {
		return ""
	}
	return "检测到系统里已有其他 cloudflared 进程。OpenWatcher 会继续使用独立实例，不会复用现有配置。"
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

func firstNonBlank(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloudflaredRunArgs(store *Store, binding Binding, token string, originURL string) ([]string, error) {
	args := []string{
		"tunnel",
		"--no-autoupdate",
		"--loglevel", "info",
		"--transport-loglevel", "warn",
	}
	if _, statErr := os.Stat(store.CredentialsPath()); statErr == nil {
		return append(args, "--config", store.RunnerConfigPath(), "run", binding.TunnelID), nil
	}
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("managed tunnel credentials are missing")
	}
	return append(args, "--url", originURL, "run", "--token-file", store.TokenPath()), nil
}
