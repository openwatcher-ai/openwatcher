package main

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/devenv"
	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/settings"
	"openwatcher/desktop-app/internal/widget"
	"openwatcher/desktop-app/internal/widgetauth"
	rootconfig "openwatcher/internal/config"
	"openwatcher/internal/pairing"
)

func TestStartupSkipsBackendWhenAutoStartDisabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	if err := settings.SaveDesktopSettings(settings.DesktopSettings{AutoStartBackend: false}); err != nil {
		t.Fatalf("SaveDesktopSettings err = %v", err)
	}

	app := &App{
		backendManager: backend.NewManager(backend.NewBinaryLocator(t.TempDir()), logging.NewRedactor()),
	}
	app.startup(context.Background())

	if got := len(app.backendManager.GetBackendLogs(10)); got != 0 {
		t.Fatalf("backend logs len = %d, want 0 when auto start disabled", got)
	}
}

func TestStartupAttemptsBackendWhenAutoStartEnabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	if err := settings.SaveDesktopSettings(settings.DesktopSettings{AutoStartBackend: true}); err != nil {
		t.Fatalf("SaveDesktopSettings err = %v", err)
	}

	app := &App{
		backendManager: backend.NewManager(backend.NewBinaryLocator(t.TempDir()), logging.NewRedactor()),
	}
	app.startup(context.Background())

	if got := len(app.backendManager.GetBackendLogs(10)); got == 0 {
		t.Fatalf("backend logs len = 0, want attempted startup log when auto start enabled")
	}
}

func TestStartupRestoresDeveloperEnvironmentWhenEnabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	repoDir := t.TempDir()
	if err := settings.SaveDesktopSettings(settings.DesktopSettings{
		AutoStartBackend: false,
		DeveloperEnvironment: settings.DeveloperEnvironmentSettings{
			Enabled:    true,
			Mode:       "workspace",
			RepoPath:   repoDir,
			BaseURL:    "http://10.0.2.2:19876",
			DeviceName: "watch",
			HostAlias:  "10.0.2.2",
		},
	}); err != nil {
		t.Fatalf("SaveDesktopSettings err = %v", err)
	}

	app := &App{
		backendManager: backend.NewManager(backend.NewBinaryLocator(t.TempDir()), logging.NewRedactor()),
		devEnvManager:  devenv.NewManagerWithLogPath(logging.NewRedactor(), ""),
	}
	app.startup(context.Background())

	status := app.devEnvManager.Status()
	if !status.Enabled {
		t.Fatalf("developer status enabled = false, want true")
	}
	if status.Config.RepoPath != repoDir {
		t.Fatalf("developer repo path = %q, want %q", status.Config.RepoPath, repoDir)
	}
	if status.LastError == "" {
		t.Fatalf("developer last error empty, want attempted startup result")
	}
}

func TestSetAutoStartBackendPersistsValue(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	app := &App{}
	saved, err := app.SetAutoStartBackend(false)
	if err != nil {
		t.Fatalf("SetAutoStartBackend err = %v", err)
	}
	if saved.AutoStartBackend {
		t.Fatalf("SetAutoStartBackend returned true, want false")
	}
	if saved.DeveloperEnvironment.DeviceName != "watch" {
		t.Fatalf("SetAutoStartBackend lost developer defaults: %#v", saved.DeveloperEnvironment)
	}

	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		t.Fatalf("LoadDesktopSettings err = %v", err)
	}
	if loaded.AutoStartBackend {
		t.Fatalf("LoadDesktopSettings autoStartBackend = true, want false")
	}
}

func TestFloatingWidgetTogglePreparesCredentialBeforePersistingEnabled(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	backendConfig := filepath.Join(configHome, "sidecar.json")
	t.Setenv("OPENWATCHER_CONFIG", backendConfig)
	if err := settings.SaveDesktopSettings(settings.DesktopSettings{AutoStartBackend: false, FloatingWidgetEnabled: false}); err != nil {
		t.Fatal(err)
	}
	store := &widgetauth.MemoryStore{}
	app := &App{widgetStore: store, widgetSync: &widgetauth.ConfigSynchronizer{}}
	state, err := app.SetFloatingWidgetEnabled(true)
	if err != nil {
		t.Fatal(err)
	}
	if !state.FloatingWidgetEnabled {
		t.Fatal("enabled preference was not returned")
	}
	token, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := rootconfig.Load(backendConfig)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WidgetTokenHash != pairing.HashToken(token) || app.widgetListenAddress() != "127.0.0.1:0" {
		t.Fatalf("credential was not ready before enable: cfg=%+v listen=%q", cfg, app.widgetListenAddress())
	}
	loaded, err := settings.LoadDesktopSettings()
	if err != nil || !loaded.FloatingWidgetEnabled {
		t.Fatalf("saved settings = %+v, %v", loaded, err)
	}
}

func TestFloatingWidgetCredentialFailureKeepsDisabledPreference(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	t.Setenv("OPENWATCHER_CONFIG", filepath.Join(configHome, "sidecar.json"))
	if err := settings.SaveDesktopSettings(settings.DesktopSettings{AutoStartBackend: false, FloatingWidgetEnabled: false}); err != nil {
		t.Fatal(err)
	}
	app := &App{widgetStore: invalidWidgetStore{}, widgetSync: &widgetauth.ConfigSynchronizer{}}
	if _, err := app.SetFloatingWidgetEnabled(true); !errors.Is(err, widgetauth.ErrInvalidCredential) {
		t.Fatalf("SetFloatingWidgetEnabled err = %v", err)
	}
	loaded, err := settings.LoadDesktopSettings()
	if err != nil || loaded.FloatingWidgetEnabled {
		t.Fatalf("failed enable was persisted: %+v, %v", loaded, err)
	}
	if app.widgetListenAddress() != "" {
		t.Fatal("failed credential exposed a widget listener")
	}
}

func TestRepairFloatingWidgetCredentialRotatesTokenAndHash(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)
	backendConfig := filepath.Join(configHome, "sidecar.json")
	t.Setenv("OPENWATCHER_CONFIG", backendConfig)
	if err := settings.SaveDesktopSettings(settings.DesktopSettings{AutoStartBackend: false, FloatingWidgetEnabled: true}); err != nil {
		t.Fatal(err)
	}
	store := &widgetauth.MemoryStore{}
	app := &App{widgetStore: store, widgetSync: &widgetauth.ConfigSynchronizer{}}
	if err := app.prepareWidgetCredential(); err != nil {
		t.Fatal(err)
	}
	previous, _ := store.Read()
	if _, err := app.RepairFloatingWidgetCredential(); err != nil {
		t.Fatal(err)
	}
	current, err := store.Read()
	if err != nil || current == previous {
		t.Fatalf("credential was not rotated: previous=%q current=%q err=%v", previous, current, err)
	}
	cfg, _, err := rootconfig.Load(backendConfig)
	if err != nil || cfg.WidgetTokenHash != pairing.HashToken(current) {
		t.Fatalf("repaired hash mismatch: %+v, %v", cfg, err)
	}
}

func TestStartConfigIncludesWidgetListenerOnlyAfterCredentialIsReady(t *testing.T) {
	app := &App{}
	if got := app.startConfigFromRequest(BackendRequest{Mode: "lan"}).WidgetListen; got != "" {
		t.Fatalf("listener before credential = %q", got)
	}
	app.widgetStateMu.Lock()
	app.widgetReady = true
	app.widgetStateMu.Unlock()
	if got := app.startConfigFromRequest(BackendRequest{Mode: "lan"}).WidgetListen; got != "127.0.0.1:0" {
		t.Fatalf("listener after credential = %q", got)
	}
}

func TestShutdownStopsWidgetBeforeCancellingProcessContext(t *testing.T) {
	processCtx, cancel := context.WithCancel(context.Background())
	process := &shutdownOrderProcess{ctx: processCtx, done: make(chan struct{})}
	manager := widget.NewManagerWithDependencies(
		func() (string, error) { return "helper", nil },
		func(string, string, ...string) (widget.Process, error) { return process, nil },
		fixedAppWidgetToken{},
		func(time.Duration) {},
		time.Now,
	)
	if err := manager.Enable("http://127.0.0.1:8787"); err != nil {
		t.Fatal(err)
	}
	app := &App{processCtx: processCtx, processCancel: cancel, widgetManager: manager}
	app.shutdown(context.Background())
	if process.cancelledAtKill {
		t.Fatal("process context was cancelled before Widget stop")
	}
	if processCtx.Err() == nil {
		t.Fatal("process context was not cancelled after shutdown")
	}
}

type invalidWidgetStore struct{}

func (invalidWidgetStore) Read() (string, error) { return "corrupted", nil }
func (invalidWidgetStore) Write(string) error    { return nil }
func (invalidWidgetStore) Delete() error         { return nil }

type fixedAppWidgetToken struct{}

func (fixedAppWidgetToken) Token() (string, error) { return "private-token", nil }

type shutdownOrderProcess struct {
	ctx             context.Context
	done            chan struct{}
	once            sync.Once
	cancelledAtKill bool
}

func (p *shutdownOrderProcess) Kill() error {
	p.cancelledAtKill = p.ctx.Err() != nil
	p.once.Do(func() { close(p.done) })
	return nil
}
func (p *shutdownOrderProcess) Wait() error {
	<-p.done
	return nil
}

func TestStopDeveloperEnvironmentDisablesRestorePreference(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("HOME", configHome)

	app := &App{
		devEnvManager: devenv.NewManagerWithLogPath(logging.NewRedactor(), ""),
	}
	_ = app.persistDeveloperEnvironmentPreference(DeveloperEnvironmentRequest{
		Enabled:    true,
		Mode:       "workspace",
		RepoPath:   t.TempDir(),
		BaseURL:    "http://10.0.2.2:18787",
		DeviceName: "watch",
		HostAlias:  "10.0.2.2",
	}, true)

	app.StopDeveloperEnvironment()

	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		t.Fatalf("LoadDesktopSettings err = %v", err)
	}
	if loaded.DeveloperEnvironment.Enabled {
		t.Fatalf("developer restore enabled = true, want false")
	}
}
