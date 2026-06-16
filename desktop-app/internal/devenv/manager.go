package devenv

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/processutil"
	"openwatcher/desktop-app/internal/settings"
)

const (
	defaultWorkspaceListen = "127.0.0.1:18787"
	defaultDeviceBaseURL   = "http://10.0.2.2:18787"
	workspaceStartupGrace  = 30 * time.Second
)

type Mode string

const (
	ModeWorkspace Mode = "workspace"
	ModeExternal  Mode = "external"
)

type Config struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	RepoPath      string `json:"repoPath,omitempty"`
	BaseURL       string `json:"baseUrl,omitempty"`
	DeviceName    string `json:"deviceName,omitempty"`
	ManagedTunnel bool   `json:"managedTunnelEnabled"`
	HostAlias     string `json:"hostAlias,omitempty"`
}

type Status struct {
	Enabled              bool    `json:"enabled"`
	Mode                 string  `json:"mode"`
	RepoPath             string  `json:"repoPath,omitempty"`
	ResolvedRepoPath     string  `json:"resolvedRepoPath,omitempty"`
	ResolvedScriptPath   string  `json:"resolvedScriptPath,omitempty"`
	ResolvedEnvFilePath  string  `json:"resolvedEnvFilePath,omitempty"`
	Listen               string  `json:"listen,omitempty"`
	BaseURL              string  `json:"baseUrl,omitempty"`
	HostAlias            string  `json:"hostAlias,omitempty"`
	Running              bool    `json:"running"`
	ManagedByDesktop     bool    `json:"managedByDesktop"`
	ExternallyManaged    bool    `json:"externallyManaged"`
	ManagedTunnelEnabled bool    `json:"managedTunnelEnabled"`
	State                string  `json:"state"`
	Message              string  `json:"message"`
	StartedAt            string  `json:"startedAt,omitempty"`
	RecentLogCount       int     `json:"recentLogCount"`
	LogFileLabel         string  `json:"logFileLabel,omitempty"`
	StartCommand         string  `json:"startCommand,omitempty"`
	EnvFilePresent       bool    `json:"envFilePresent"`
	LastHealth           *Health `json:"lastHealth,omitempty"`
	LastError            string  `json:"lastError,omitempty"`
	LastCheckedAt        string  `json:"lastCheckedAt,omitempty"`
	Config               Config  `json:"config"`
}

type Health struct {
	OK            bool   `json:"ok"`
	Message       string `json:"message"`
	Endpoint      string `json:"endpoint,omitempty"`
	Listen        string `json:"listen,omitempty"`
	PublicBaseURL string `json:"publicBaseUrl,omitempty"`
	CheckedAt     string `json:"checkedAt,omitempty"`
}

type LogLine struct {
	At      string `json:"at"`
	Message string `json:"message"`
}

type Repository struct {
	Path         string `json:"path"`
	Label        string `json:"label"`
	AutoDetected bool   `json:"autoDetected"`
	Valid        bool   `json:"valid"`
	Message      string `json:"message,omitempty"`
}

type Manager struct {
	redactor *logging.Redactor
	logPath  string
	goLookup func() (string, error)

	mu         sync.Mutex
	cmd        *exec.Cmd
	logs       []LogLine
	startedAt  time.Time
	lastHealth *Health
	lastError  string
	lastConfig Config
}

func NewManager(redactor *logging.Redactor) *Manager {
	return NewManagerWithLogPath(redactor, defaultLogPath())
}

func NewManagerWithLogPath(redactor *logging.Redactor, logPath string) *Manager {
	return &Manager{
		redactor: redactor,
		logPath:  strings.TrimSpace(logPath),
		goLookup: discoverGoBinary,
	}
}

func DetectRepositories(currentRepoRoot string) []Repository {
	trimmed := strings.TrimSpace(currentRepoRoot)
	if trimmed == "" {
		return nil
	}
	repo := Repository{
		Path:         trimmed,
		Label:        filepath.Base(trimmed),
		AutoDetected: true,
	}
	if scriptPath, err := workspaceScriptPath(trimmed); err == nil {
		repo.Valid = true
		repo.Message = filepath.Base(scriptPath)
	} else {
		repo.Valid = false
		repo.Message = err.Error()
	}
	return []Repository{repo}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) GetLogs(limit int) []LogLine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || len(m.logs) <= limit {
		return append([]LogLine(nil), m.logs...)
	}
	return append([]LogLine(nil), m.logs[len(m.logs)-limit:]...)
}

func (m *Manager) Ensure(ctx context.Context, cfg Config) Status {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return m.invalidStatus(cfg, err)
	}

	if !normalized.Enabled {
		_ = m.Stop(ctx)
		m.mu.Lock()
		m.lastConfig = normalized
		m.lastError = ""
		m.lastHealth = nil
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}

	switch Mode(normalized.Mode) {
	case ModeExternal:
		_ = m.Stop(ctx)
		health := m.checkHealth(ctx, normalized)
		m.mu.Lock()
		m.lastConfig = normalized
		m.lastHealth = &health
		m.lastError = ""
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	default:
		health := m.checkHealth(ctx, normalized)
		if health.OK && !m.isManagedProcessRunning() {
			m.mu.Lock()
			m.lastConfig = normalized
			m.lastHealth = &health
			m.lastError = ""
			status := m.statusLocked()
			m.mu.Unlock()
			return status
		}

		m.mu.Lock()
		needsRestart := configChanged(m.lastConfig, normalized) || m.cmd == nil || m.cmd.Process == nil
		m.mu.Unlock()
		if needsRestart {
			if err := m.startWorkspace(ctx, normalized); err != nil {
				m.mu.Lock()
				m.lastConfig = normalized
				m.lastError = err.Error()
				m.lastHealth = &Health{
					OK:        false,
					Message:   err.Error(),
					Endpoint:  normalizeLoopbackHealthEndpoint(normalized.BaseURL),
					CheckedAt: time.Now().Format(time.RFC3339),
				}
				status := m.statusLocked()
				m.mu.Unlock()
				return status
			}
		}

		health = m.checkHealth(ctx, normalized)
		if !health.OK {
			if m.shouldRestartWorkspaceAfterFailedHealth() {
				if restartErr := m.startWorkspace(ctx, normalized); restartErr == nil {
					health = m.checkHealth(ctx, normalized)
				} else {
					health = Health{
						OK:        false,
						Message:   restartErr.Error(),
						Endpoint:  normalizeLoopbackHealthEndpoint(normalized.BaseURL),
						CheckedAt: time.Now().Format(time.RFC3339),
					}
				}
			}
		}
		m.mu.Lock()
		m.lastConfig = normalized
		m.lastHealth = &health
		if health.OK {
			m.lastError = ""
		} else {
			m.lastError = health.Message
		}
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}
}

func (m *Manager) Observe(ctx context.Context, cfg Config) Status {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return m.invalidStatus(cfg, err)
	}

	if !normalized.Enabled {
		m.mu.Lock()
		m.lastConfig = normalized
		m.lastError = ""
		m.lastHealth = nil
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}

	switch Mode(normalized.Mode) {
	case ModeExternal:
		health := m.checkHealth(ctx, normalized)
		m.mu.Lock()
		m.lastConfig = normalized
		m.lastHealth = &health
		if health.OK {
			m.lastError = ""
		} else {
			m.lastError = health.Message
		}
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	default:
		m.mu.Lock()
		running := m.cmd != nil && m.cmd.Process != nil
		m.lastConfig = normalized
		m.mu.Unlock()
		health := m.checkHealth(ctx, normalized)
		m.mu.Lock()
		m.lastHealth = &health
		if health.OK {
			m.lastError = ""
		} else if running {
			m.lastError = health.Message
		}
		status := m.statusLocked()
		m.mu.Unlock()
		return status
	}
}

func (m *Manager) Stop(context.Context) error {
	m.mu.Lock()
	if m.cmd != nil && m.cmd.Process != nil {
		err := m.cmd.Process.Kill()
		m.cmd = nil
		m.startedAt = time.Time{}
		m.lastError = ""
		m.lastHealth = nil
		m.appendLocked("开发环境已停止")
		m.mu.Unlock()
		return err
	}
	listen := loopbackListenForBaseURL(m.lastConfig.BaseURL)
	m.startedAt = time.Time{}
	m.lastError = ""
	m.lastHealth = nil
	m.mu.Unlock()
	if listen == "" {
		return nil
	}
	killed, err := processutil.KillListeningProcess(listen)
	m.mu.Lock()
	defer m.mu.Unlock()
	if killed {
		m.appendLocked("开发环境已停止")
		return nil
	}
	if err != nil {
		m.lastError = err.Error()
		return err
	}
	return nil
}

func (m *Manager) ClearLogs() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = nil
	if strings.TrimSpace(m.logPath) != "" {
		_ = os.Remove(m.logPath)
	}
}

func (m *Manager) LogPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.TrimSpace(m.logPath)
}

func (m *Manager) isManagedProcessRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && m.cmd.Process != nil
}

func (m *Manager) StopListeningProcess(baseURL string) error {
	listen := loopbackListenForBaseURL(baseURL)
	if listen == "" {
		return nil
	}
	_, err := processutil.KillListeningProcess(listen)
	return err
}

func (m *Manager) startWorkspace(ctx context.Context, cfg Config) error {
	repoPath, err := resolveRepoPath(cfg.RepoPath)
	if err != nil {
		return err
	}
	scriptPath, err := workspaceScriptPath(repoPath)
	if err != nil {
		return err
	}
	goBinary, err := m.resolveGoBinary()
	if err != nil {
		return err
	}

	_ = m.Stop(ctx)
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(),
		"OPENWATCHER_CONFIG="+os.ExpandEnv("$HOME/.openwatcher/config.json"),
		"OPENWATCHER_LISTEN="+loopbackListenForBaseURL(cfg.BaseURL),
		"OPENWATCHER_PUBLIC_BASE_URL="+cfg.BaseURL,
		"OPENWATCHER_PAIRING_SLOT=dev",
		"OPENWATCHER_GO_BIN="+goBinary,
		"PATH="+prependPathDir(os.Getenv("PATH"), filepath.Dir(goBinary)),
	)
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

	m.mu.Lock()
	m.cmd = cmd
	m.startedAt = time.Now()
	m.lastConfig = cfg
	m.appendLocked("开发环境启动：" + trimRepoPath(repoPath))
	m.appendLocked("开发环境启动脚本：" + startCommandLabel(repoPath, scriptPath))
	m.appendLocked("开发环境 Go 工具链：" + labelHomePath(goBinary))
	m.appendLocked("开发环境健康检查地址：" + normalizeLoopbackHealthEndpoint(cfg.BaseURL))
	m.mu.Unlock()
	go m.captureStream(stdout)
	go m.captureStream(stderr)
	go m.waitForExit(cmd)
	return nil
}

func (m *Manager) waitForExit(cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != cmd {
		return
	}
	m.cmd = nil
	if err == nil {
		m.lastError = "开发环境进程已退出"
		m.appendLocked("开发环境退出：进程已结束")
		return
	}
	m.lastError = classifyWorkspaceError(err.Error(), m.logs)
	m.appendLocked("开发环境退出：" + m.lastError)
}

func (m *Manager) captureStream(stream io.Reader) {
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
	line := LogLine{
		At:      time.Now().Format(time.RFC3339),
		Message: trimmed,
	}
	m.logs = append(m.logs, line)
	if len(m.logs) > 300 {
		m.logs = append([]LogLine(nil), m.logs[len(m.logs)-300:]...)
	}
	m.writeLine(line)
}

func (m *Manager) statusLocked() Status {
	cfg := m.lastConfig
	resolvedRepoPath, _ := resolveRepoPath(cfg.RepoPath)
	scriptPath, _ := workspaceScriptPath(resolvedRepoPath)
	envFilePath := EnvFilePath(resolvedRepoPath)
	managedRunning := m.cmd != nil && m.cmd.Process != nil
	externalRunning := !managedRunning && Mode(cfg.Mode) == ModeWorkspace && cfg.Enabled && m.lastHealth != nil && m.lastHealth.OK
	status := Status{
		Enabled:              cfg.Enabled,
		Mode:                 cfg.Mode,
		RepoPath:             cfg.RepoPath,
		ResolvedRepoPath:     resolvedRepoPath,
		ResolvedScriptPath:   scriptPath,
		ResolvedEnvFilePath:  envFilePath,
		Listen:               loopbackListenForBaseURL(cfg.BaseURL),
		BaseURL:              cfg.BaseURL,
		HostAlias:            cfg.HostAlias,
		Running:              managedRunning || externalRunning,
		ManagedByDesktop:     managedRunning,
		ExternallyManaged:    externalRunning,
		ManagedTunnelEnabled: cfg.ManagedTunnel,
		StartedAt:            startedAtLabel(m.startedAt, managedRunning),
		RecentLogCount:       len(m.logs),
		LogFileLabel:         labelHomePath(m.logPath),
		StartCommand:         startCommandLabel(resolvedRepoPath, scriptPath),
		EnvFilePresent:       fileExists(envFilePath),
		LastHealth:           m.lastHealth,
		LastError:            m.lastError,
		Config:               cfg,
	}
	switch {
	case !cfg.Enabled:
		status.State = "disabled"
		status.Message = "开发环境未启用"
	case Mode(cfg.Mode) == ModeExternal:
		if m.lastHealth != nil && m.lastHealth.OK {
			status.State = "healthy"
			status.Message = "外部开发环境在线"
		} else if m.lastHealth != nil {
			status.State = "error"
			status.Message = m.lastHealth.Message
		} else {
			status.State = "pending"
			status.Message = "等待外部开发环境检测"
		}
	default:
		if status.Running && m.lastHealth != nil && m.lastHealth.OK {
			status.State = "healthy"
			if status.ManagedByDesktop {
				status.Message = "开发环境运行中"
			} else {
				status.Message = "已检测到本机开发环境"
			}
		} else if managedRunning {
			status.State = "recovering"
			if workspaceStartingSince(m.startedAt) {
				status.Message = "开发环境启动中"
			} else {
				status.Message = firstNonBlank(m.lastError, m.lastHealthMessage(), "开发环境启动中")
			}
		} else if strings.TrimSpace(m.lastError) != "" {
			status.State = "error"
			status.Message = m.lastError
		} else {
			status.State = "pending"
			status.Message = "开发环境未启动"
		}
	}
	if m.lastHealth != nil {
		status.LastCheckedAt = m.lastHealth.CheckedAt
	}
	return status
}

func (m *Manager) lastHealthMessage() string {
	if m.lastHealth == nil {
		return ""
	}
	return m.lastHealth.Message
}

func (m *Manager) checkHealth(ctx context.Context, cfg Config) Health {
	endpoint := normalizeLoopbackHealthEndpoint(cfg.BaseURL)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		m.recordLog("开发环境健康检查构造失败：" + endpoint)
		return Health{OK: false, Message: "构造开发环境健康检查失败", Endpoint: endpoint, CheckedAt: time.Now().Format(time.RFC3339)}
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		m.recordLog("开发环境健康检查未连通：" + endpoint)
		return Health{OK: false, Message: "尚未连接到开发环境 /healthz", Endpoint: endpoint, CheckedAt: time.Now().Format(time.RFC3339)}
	}
	defer response.Body.Close()
	ok := response.StatusCode >= 200 && response.StatusCode < 300
	if ok {
		m.recordLog(fmt.Sprintf("开发环境健康检查通过：HTTP %d", response.StatusCode))
	} else {
		m.recordLog(fmt.Sprintf("开发环境健康检查失败：HTTP %d", response.StatusCode))
	}
	return Health{
		OK:            ok,
		Message:       fmt.Sprintf("HTTP %d", response.StatusCode),
		Endpoint:      endpoint,
		Listen:        loopbackListenForBaseURL(cfg.BaseURL),
		PublicBaseURL: cfg.BaseURL,
		CheckedAt:     time.Now().Format(time.RFC3339),
	}
}

func normalizeConfig(cfg Config) (Config, error) {
	cfg.Mode = strings.TrimSpace(cfg.Mode)
	if cfg.Mode == "" {
		cfg.Mode = string(ModeWorkspace)
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultDeviceBaseURL
	}
	if _, err := url.ParseRequestURI(cfg.BaseURL); err != nil {
		return Config{}, fmt.Errorf("开发环境地址不合法")
	}
	cfg.HostAlias = strings.TrimSpace(cfg.HostAlias)
	switch Mode(cfg.Mode) {
	case ModeWorkspace:
		if _, err := resolveRepoPath(cfg.RepoPath); err != nil {
			return Config{}, err
		}
	case ModeExternal:
	default:
		return Config{}, fmt.Errorf("未知开发环境模式：%s", cfg.Mode)
	}
	return cfg, nil
}

func resolveRepoPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("请先选择开发仓库")
	}
	resolved, err := filepath.Abs(trimmed)
	if err != nil {
		return "", errors.New("开发仓库路径无法解析")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("开发仓库路径不存在")
	}
	return resolved, nil
}

func workspaceScriptPath(repoPath string) (string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("开发仓库路径为空")
	}
	scriptPath := filepath.Join(repoPath, "scripts", "start-local.sh")
	info, err := os.Stat(scriptPath)
	if err != nil || info.IsDir() {
		return "", errors.New("所选仓库缺少 scripts/start-local.sh")
	}
	return scriptPath, nil
}

func configChanged(before, after Config) bool {
	return before.Enabled != after.Enabled ||
		before.Mode != after.Mode ||
		filepath.Clean(strings.TrimSpace(before.RepoPath)) != filepath.Clean(strings.TrimSpace(after.RepoPath)) ||
		strings.TrimSpace(before.BaseURL) != strings.TrimSpace(after.BaseURL) ||
		before.ManagedTunnel != after.ManagedTunnel ||
		strings.TrimSpace(before.HostAlias) != strings.TrimSpace(after.HostAlias)
}

func normalizeLoopbackHealthEndpoint(baseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = defaultDeviceBaseURL
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "http://" + defaultWorkspaceListen + "/healthz"
	}
	port := parsed.Port()
	if port == "" {
		port = defaultWorkspacePort()
	}
	parsed.Scheme = "http"
	// 开发环境始终是 Desktop 本地进程，健康检查和监听地址都必须回到 loopback。
	parsed.Host = net.JoinHostPort("127.0.0.1", port)
	parsed.Path = "/healthz"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func defaultWorkspacePort() string {
	_, port, err := net.SplitHostPort(defaultWorkspaceListen)
	if err != nil || strings.TrimSpace(port) == "" {
		return "18787"
	}
	return port
}

func loopbackListenForBaseURL(baseURL string) string {
	endpoint := normalizeLoopbackHealthEndpoint(baseURL)
	parsed, err := url.Parse(endpoint)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return defaultWorkspaceListen
	}
	return parsed.Host
}

func (m *Manager) invalidStatus(cfg Config, err error) Status {
	m.mu.Lock()
	m.lastConfig = cfg
	m.lastError = err.Error()
	m.appendLocked("开发环境配置错误：" + err.Error())
	m.lastHealth = &Health{
		OK:        false,
		Message:   err.Error(),
		CheckedAt: time.Now().Format(time.RFC3339),
	}
	status := m.statusLocked()
	m.mu.Unlock()
	return status
}

func (m *Manager) shouldRestartWorkspaceAfterFailedHealth() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.cmd.Process == nil {
		return true
	}
	return !workspaceStartingSince(m.startedAt)
}

func (m *Manager) resolveGoBinary() (string, error) {
	if m.goLookup == nil {
		return discoverGoBinary()
	}
	found, err := m.goLookup()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("未找到 Go 命令。请先安装 Go，或把 go 加入 PATH 后重新打开 Desktop。")
		}
		return "", err
	}
	return found, nil
}

func (m *Manager) recordLog(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appendLocked(message)
}

func (m *Manager) writeLine(line LogLine) {
	if strings.TrimSpace(m.logPath) == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(m.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "[%s] %s\n", line.At, line.Message)
}

func defaultLogPath() string {
	configDir, err := settings.ConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configDir, "logs", "developer-environment.log")
}

func labelHomePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

func workspaceStartingSince(startedAt time.Time) bool {
	if startedAt.IsZero() {
		return false
	}
	return time.Since(startedAt) < workspaceStartupGrace
}

func discoverGoBinary() (string, error) {
	if found, err := exec.LookPath("go"); err == nil && strings.TrimSpace(found) != "" {
		return found, nil
	}

	homeDir, _ := os.UserHomeDir()
	candidates := []string{
		"/opt/homebrew/bin/go",
		"/usr/local/bin/go",
		"/usr/local/go/bin/go",
	}
	if strings.TrimSpace(homeDir) != "" {
		candidates = append(candidates,
			filepath.Join(homeDir, ".asdf", "shims", "go"),
			filepath.Join(homeDir, ".local", "bin", "go"),
		)
	}
	for _, candidate := range candidates {
		if isExecutableFile(candidate) {
			return candidate, nil
		}
	}

	shellCandidates := []string{}
	if shell := strings.TrimSpace(os.Getenv("SHELL")); shell != "" {
		shellCandidates = append(shellCandidates, shell)
	}
	for _, shell := range []string{"/bin/zsh", "/bin/bash", "/bin/sh"} {
		if !containsString(shellCandidates, shell) {
			shellCandidates = append(shellCandidates, shell)
		}
	}
	for _, shell := range shellCandidates {
		if !isExecutableFile(shell) {
			continue
		}
		output, err := exec.Command(shell, "-lc", "command -v go").Output()
		if err != nil {
			continue
		}
		found := strings.TrimSpace(string(output))
		if isExecutableFile(found) {
			return found, nil
		}
	}
	return "", errors.New("未找到 Go 命令。请先安装 Go，或把 go 加入 PATH 后重新打开 Desktop。")
}

func isExecutableFile(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func prependPathDir(pathValue, dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return pathValue
	}
	if pathValue == "" {
		return dir
	}
	parts := strings.Split(pathValue, string(os.PathListSeparator))
	for _, part := range parts {
		if strings.TrimSpace(part) == dir {
			return pathValue
		}
	}
	return dir + string(os.PathListSeparator) + pathValue
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func classifyWorkspaceError(exitMessage string, logs []LogLine) string {
	candidates := []string{strings.TrimSpace(exitMessage)}
	for _, line := range logs {
		candidates = append(candidates, strings.TrimSpace(line.Message))
	}
	for _, candidate := range candidates {
		lower := strings.ToLower(candidate)
		switch {
		case strings.Contains(lower, "go: command not found"):
			return "未找到 Go 命令。请先安装 Go，或把 go 加入 PATH 后重新打开 Desktop。"
		case strings.Contains(lower, "openwatcher_go_bin"), strings.Contains(lower, "no such file or directory"):
			if strings.Contains(lower, "go") {
				return "未找到可用的 Go 工具链。请确认 Go 已安装，并重新打开 Desktop。"
			}
		}
	}
	return strings.TrimSpace(exitMessage)
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func startedAtLabel(startedAt time.Time, running bool) string {
	if !running || startedAt.IsZero() {
		return ""
	}
	return startedAt.Format(time.RFC3339)
}

func trimRepoPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Base(path)
}

func startCommandLabel(repoPath, scriptPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	scriptPath = strings.TrimSpace(scriptPath)
	if repoPath == "" || scriptPath == "" {
		return ""
	}
	relative, err := filepath.Rel(repoPath, scriptPath)
	if err == nil && strings.TrimSpace(relative) != "" && !strings.HasPrefix(relative, "..") {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(scriptPath)
}

func EnvFilePath(repoPath string) string {
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		return ""
	}
	return filepath.Join(repoPath, ".env.development")
}

func fileExists(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}
