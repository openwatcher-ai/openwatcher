package main

import (
	"context"
	"testing"

	"openwatcher/desktop-app/internal/backend"
	"openwatcher/desktop-app/internal/devenv"
	"openwatcher/desktop-app/internal/logging"
	"openwatcher/desktop-app/internal/settings"
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
