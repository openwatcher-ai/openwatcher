package main

import (
	"context"
	"testing"

	"openwatcher/desktop-app/internal/backend"
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

	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		t.Fatalf("LoadDesktopSettings err = %v", err)
	}
	if loaded.AutoStartBackend {
		t.Fatalf("LoadDesktopSettings autoStartBackend = true, want false")
	}
}
