// Package widget supervises the non-sensitive Widget helper process.
package widget

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"openwatcher/desktop-app/internal/processutil"
	"openwatcher/desktop-app/internal/settings"
	"openwatcher/desktop-app/internal/widgettransport"
)

var ErrHelperMissing = errors.New("OpenWatcher 悬浮球辅助程序未找到")

type Process interface {
	Kill() error
	Wait() error
}

type Starter func(path, token string, args ...string) (Process, error)
type Locator func() (string, error)
type TokenSource interface{ Token() (string, error) }

type Status struct {
	Enabled         bool   `json:"enabled"`
	Running         bool   `json:"running"`
	Endpoint        string `json:"endpoint,omitempty"`
	RestartAttempts int    `json:"restartAttempts"`
	Message         string `json:"message,omitempty"`
}

type Manager struct {
	mu         sync.Mutex
	locate     Locator
	start      Starter
	tokens     TokenSource
	sleep      func(time.Duration)
	now        func() time.Time
	process    Process
	endpoint   string
	enabled    bool
	attempts   []time.Time
	message    string
	generation uint64
}

func NewManager(appRoot string, tokens TokenSource) *Manager {
	return NewManagerWithDependencies(defaultLocator(appRoot), startProcess, tokens, time.Sleep, time.Now)
}

func NewManagerWithDependencies(l Locator, s Starter, tokens TokenSource, sleep func(time.Duration), now func() time.Time) *Manager {
	if sleep == nil {
		sleep = time.Sleep
	}
	if now == nil {
		now = time.Now
	}
	return &Manager{locate: l, start: s, tokens: tokens, sleep: sleep, now: now}
}

// Enable is an explicit user action and resets a previous crash limit.
func (m *Manager) Enable(endpoint string) error {
	return m.activate(endpoint, true)
}

// Resume follows the Desktop/backend lifecycle without discarding recent
// crash history.
func (m *Manager) Resume(endpoint string) error {
	return m.activate(endpoint, false)
}

func (m *Manager) activate(endpoint string, resetAttempts bool) error {
	canonical, err := widgettransport.ParseEndpoint(endpoint)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if resetAttempts {
		m.attempts = nil
	}
	m.enabled = true
	if m.process != nil && m.endpoint == canonical {
		m.message = ""
		m.mu.Unlock()
		return nil
	}
	old := m.process
	m.process = nil
	m.endpoint = canonical
	m.message = ""
	m.generation++
	generation := m.generation
	m.mu.Unlock()
	if old != nil {
		_ = old.Kill()
	}
	return m.startGeneration(canonical, generation)
}

func (m *Manager) UpdateEndpoint(endpoint string) error {
	canonical, err := widgettransport.ParseEndpoint(endpoint)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return nil
	}
	if m.endpoint == canonical && m.process != nil {
		m.mu.Unlock()
		return nil
	}
	old := m.process
	m.process = nil
	m.endpoint = canonical
	m.message = ""
	m.generation++
	generation := m.generation
	m.mu.Unlock()
	if old != nil {
		_ = old.Kill()
	}
	return m.startGeneration(canonical, generation)
}

func (m *Manager) startGeneration(endpoint string, generation uint64) error {
	if m.tokens == nil {
		err := errors.New("悬浮球凭据不可用")
		m.recordFailure(endpoint, generation, err.Error())
		return err
	}
	token, err := m.tokens.Token()
	if err != nil {
		m.recordFailure(endpoint, generation, "悬浮球凭据不可用")
		return err
	}
	path, err := m.locate()
	if err != nil {
		m.recordFailure(endpoint, generation, ErrHelperMissing.Error())
		return err
	}
	p, err := m.start(path, token, "--endpoint", endpoint)
	if err != nil {
		m.recordFailure(endpoint, generation, "悬浮球启动失败")
		return err
	}
	m.mu.Lock()
	if !m.enabled || generation != m.generation {
		m.mu.Unlock()
		_ = p.Kill()
		return nil
	}
	m.process = p
	m.endpoint = endpoint
	m.message = ""
	m.mu.Unlock()
	go m.watch(p, endpoint, generation)
	return nil
}

func (m *Manager) watch(p Process, endpoint string, generation uint64) {
	_ = p.Wait()
	m.mu.Lock()
	if m.process != p || generation != m.generation {
		m.mu.Unlock()
		return
	}
	m.process = nil
	m.mu.Unlock()
	m.recordFailure(endpoint, generation, "悬浮球意外退出")
}

func (m *Manager) recordFailure(endpoint string, generation uint64, reason string) {
	m.mu.Lock()
	if !m.enabled || generation != m.generation {
		m.mu.Unlock()
		return
	}
	now := m.now()
	m.attempts = filterAfter(m.attempts, now.Add(-5*time.Minute))
	if len(m.attempts) >= 3 {
		m.message = "悬浮球已连续退出，已停止自动重试"
		m.mu.Unlock()
		return
	}
	m.attempts = append(m.attempts, now)
	attempt := len(m.attempts)
	m.message = reason + "，正在重试"
	m.mu.Unlock()

	delay := []time.Duration{time.Second, 5 * time.Second, 15 * time.Second}[attempt-1]
	go func() {
		m.sleep(delay)
		m.mu.Lock()
		active := m.enabled && m.generation == generation && m.process == nil && m.endpoint == endpoint
		m.mu.Unlock()
		if active {
			_ = m.startGeneration(endpoint, generation)
		}
	}()
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	m.enabled = false
	m.generation++
	p := m.process
	m.process = nil
	m.endpoint = ""
	m.message = ""
	m.mu.Unlock()
	if p != nil {
		return p.Kill()
	}
	return nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts = filterAfter(m.attempts, m.now().Add(-5*time.Minute))
	return Status{
		Enabled:         m.enabled,
		Running:         m.process != nil,
		Endpoint:        m.endpoint,
		RestartAttempts: len(m.attempts),
		Message:         m.message,
	}
}

func filterAfter(in []time.Time, cut time.Time) []time.Time {
	out := in[:0]
	for _, value := range in {
		if value.After(cut) {
			out = append(out, value)
		}
	}
	return out
}

func defaultLocator(root string) Locator {
	return func() (string, error) {
		executable, _ := os.Executable()
		for _, candidate := range helperCandidates(root, executable, runtime.GOOS, runtime.GOARCH) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
		return "", ErrHelperMissing
	}
}

func helperCandidates(root, executable, goos, goarch string) []string {
	name := "openwatcher-widget"
	if goos == "windows" {
		name += ".exe"
	}
	platform := goos + "-" + goarch
	candidates := make([]string, 0, 12)
	if executable != "" && goos == "darwin" {
		executableDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Clean(filepath.Join(executableDir, "..", "Library", "Helpers", "OpenWatcher Widget.app", "Contents", "MacOS", name)),
		)
	}
	for _, base := range settings.BundledResourceRoots(root) {
		candidates = append(candidates,
			filepath.Join(base, "widget", platform, name),
			filepath.Join(base, name),
			filepath.Join(base, "OpenWatcher Widget.app", "Contents", "MacOS", name),
		)
	}
	if root != "" {
		candidates = append(candidates,
			filepath.Join(root, "widget", "build", "bin", "OpenWatcher Widget.app", "Contents", "MacOS", name),
			filepath.Join(root, "widget", "build", "bin", name),
		)
	}
	return candidates
}

type commandProcess struct{ cmd *exec.Cmd }

func (p commandProcess) Kill() error { return p.cmd.Process.Kill() }
func (p commandProcess) Wait() error { return p.cmd.Wait() }

func startProcess(path, token string, args ...string) (Process, error) {
	cmd := exec.Command(path, args...)
	cmd.Stdin = strings.NewReader(token + "\n")
	processutil.HideConsoleWindow(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	cmd.Stdin = nil
	return commandProcess{cmd}, nil
}
