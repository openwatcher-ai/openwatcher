package widget

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeProcess struct {
	done   chan error
	waited chan struct{}
	mu     sync.Mutex
	killed bool
}

func newFake() *fakeProcess {
	return &fakeProcess{done: make(chan error, 1), waited: make(chan struct{})}
}

func (p *fakeProcess) Kill() error {
	p.mu.Lock()
	p.killed = true
	p.mu.Unlock()
	select {
	case p.done <- nil:
	default:
	}
	return nil
}
func (p *fakeProcess) Wait() error {
	err := <-p.done
	close(p.waited)
	return err
}
func (p *fakeProcess) wasKilled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.killed
}

type managerHarness struct {
	started chan *fakeProcess
	delays  chan time.Duration
	release chan struct{}
	mu      sync.Mutex
	args    [][]string
	tokens  []string
}

type fixedTokenSource struct{ token string }

func (s fixedTokenSource) Token() (string, error) { return s.token, nil }

func newManagerHarness() (*Manager, *managerHarness) {
	h := &managerHarness{
		started: make(chan *fakeProcess, 16),
		delays:  make(chan time.Duration, 16),
		release: make(chan struct{}, 16),
	}
	m := NewManagerWithDependencies(
		func() (string, error) { return "helper", nil },
		func(_ string, token string, args ...string) (Process, error) {
			h.mu.Lock()
			h.args = append(h.args, append([]string(nil), args...))
			h.tokens = append(h.tokens, token)
			h.mu.Unlock()
			process := newFake()
			h.started <- process
			return process, nil
		},
		fixedTokenSource{token: "private-token"},
		func(delay time.Duration) {
			h.delays <- delay
			<-h.release
		},
		time.Now,
	)
	return m, h
}

func (h *managerHarness) nextProcess(t *testing.T) *fakeProcess {
	t.Helper()
	select {
	case process := <-h.started:
		return process
	case <-time.After(time.Second):
		t.Fatal("helper did not start")
		return nil
	}
}

func (h *managerHarness) nextDelay(t *testing.T, want time.Duration) {
	t.Helper()
	select {
	case got := <-h.delays:
		if got != want {
			t.Fatalf("retry delay = %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("retry was not scheduled")
	}
}

func TestManagerOnlyPassesEndpointAndRestartsForChangedEndpoint(t *testing.T) {
	m, h := newManagerHarness()
	if err := m.Enable("http://127.0.0.1:1111"); err != nil {
		t.Fatal(err)
	}
	first := h.nextProcess(t)
	h.mu.Lock()
	args := append([][]string(nil), h.args...)
	tokens := append([]string(nil), h.tokens...)
	h.mu.Unlock()
	if len(args) != 1 || len(args[0]) != 2 || args[0][0] != "--endpoint" || args[0][1] != "http://127.0.0.1:1111" {
		t.Fatalf("args=%q", args)
	}
	if len(tokens) != 1 || tokens[0] != "private-token" {
		t.Fatalf("token was not delivered separately: %q", tokens)
	}
	if err := m.UpdateEndpoint("http://127.0.0.1:2222"); err != nil {
		t.Fatal(err)
	}
	_ = h.nextProcess(t)
	if !first.wasKilled() {
		t.Fatal("endpoint change did not replace helper")
	}
	_ = m.Stop()
}

func TestManagerRejectsEndpointBeforeChangingEnabledState(t *testing.T) {
	m, _ := newManagerHarness()
	if err := m.Enable("http://127.0.0.1:1111/private"); err == nil {
		t.Fatal("invalid endpoint was accepted")
	}
	if status := m.Status(); status.Enabled || status.Running {
		t.Fatalf("invalid enable changed state: %+v", status)
	}
}

func TestManagerRetriesUnexpectedExitIncludingSuccessfulWait(t *testing.T) {
	m, h := newManagerHarness()
	if err := m.Enable("http://127.0.0.1:1111"); err != nil {
		t.Fatal(err)
	}
	first := h.nextProcess(t)
	first.done <- nil
	<-first.waited
	h.nextDelay(t, time.Second)
	h.release <- struct{}{}
	_ = h.nextProcess(t)
	if status := m.Status(); status.RestartAttempts != 1 || !status.Running {
		t.Fatalf("status after retry = %+v", status)
	}
	_ = m.Stop()
}

func TestManagerStopsAfterThreeRetriesAndExplicitEnableResets(t *testing.T) {
	m, h := newManagerHarness()
	if err := m.Enable("http://127.0.0.1:1111"); err != nil {
		t.Fatal(err)
	}
	process := h.nextProcess(t)
	for index, delay := range []time.Duration{time.Second, 5 * time.Second, 15 * time.Second} {
		process.done <- errors.New("crash")
		<-process.waited
		h.nextDelay(t, delay)
		h.release <- struct{}{}
		process = h.nextProcess(t)
		if status := m.Status(); status.RestartAttempts != index+1 {
			t.Fatalf("attempt %d status = %+v", index+1, status)
		}
	}
	process.done <- errors.New("crash")
	<-process.waited
	waitForStatus(t, m, func(status Status) bool {
		return status.RestartAttempts == 3 && !status.Running && status.Message == "悬浮球已连续退出，已停止自动重试"
	})
	if err := m.Enable("http://127.0.0.1:1111"); err != nil {
		t.Fatal(err)
	}
	_ = h.nextProcess(t)
	if status := m.Status(); status.RestartAttempts != 0 || !status.Running {
		t.Fatalf("explicit reset = %+v", status)
	}
	_ = m.Stop()
}

func TestManagerRetriesInitialStartFailure(t *testing.T) {
	delays := make(chan time.Duration, 1)
	release := make(chan struct{}, 1)
	started := make(chan *fakeProcess, 1)
	var mu sync.Mutex
	calls := 0
	m := NewManagerWithDependencies(
		func() (string, error) { return "helper", nil },
		func(_ string, _ string, _ ...string) (Process, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if calls == 1 {
				return nil, errors.New("start failed")
			}
			process := newFake()
			started <- process
			return process, nil
		},
		fixedTokenSource{token: "private-token"},
		func(delay time.Duration) { delays <- delay; <-release },
		time.Now,
	)
	if err := m.Enable("http://127.0.0.1:1111"); err == nil {
		t.Fatal("initial start error was hidden")
	}
	if got := <-delays; got != time.Second {
		t.Fatalf("delay = %v", got)
	}
	release <- struct{}{}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("initial start failure was not retried")
	}
	if status := m.Status(); !status.Running || status.RestartAttempts != 1 {
		t.Fatalf("status = %+v", status)
	}
	_ = m.Stop()
}

func TestHelperCandidatesIncludePackagedAndDevelopmentLocations(t *testing.T) {
	root := filepath.Join("tmp", "desktop-app")
	executable := filepath.Join("tmp", "OpenWatcher.app", "Contents", "MacOS", "openwatcher")
	macCandidates := helperCandidates(root, executable, "darwin", "arm64")
	wantMac := filepath.Clean(filepath.Join("tmp", "OpenWatcher.app", "Contents", "Library", "Helpers", "OpenWatcher Widget.app", "Contents", "MacOS", "openwatcher-widget"))
	if !containsPath(macCandidates, wantMac) {
		t.Fatalf("mac helper candidate missing: %q", macCandidates)
	}
	windowsCandidates := helperCandidates(root, "", "windows", "amd64")
	wantWindows := filepath.Join(root, "bundled", "widget", "windows-amd64", "openwatcher-widget.exe")
	if !containsPath(windowsCandidates, wantWindows) {
		t.Fatalf("Windows helper candidate missing: %q", windowsCandidates)
	}
}

func waitForStatus(t *testing.T, manager *Manager, match func(Status) bool) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		if match(manager.Status()) {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("status did not reach expected state: %+v", manager.Status())
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if filepath.Clean(path) == filepath.Clean(want) {
			return true
		}
	}
	return false
}
