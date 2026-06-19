package main

import (
	"context"
	"runtime"

	"openwatcher/desktop-app/internal/devenv"
	"openwatcher/desktop-app/internal/settings"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) applicationMenu() *menu.Menu {
	controlMenu := menu.NewMenu()
	controlMenu.AddText("显示 OpenWatcher", nil, func(*menu.CallbackData) {
		a.showMainWindow()
	})
	controlMenu.AddSeparator()
	controlMenu.AddText("启动本机服务", nil, func(*menu.CallbackData) {
		_ = a.StartBackend()
		a.emitDesktopStateChanged("backend-started")
	})
	controlMenu.AddText("停止本机服务", nil, func(*menu.CallbackData) {
		_ = a.StopBackend()
		a.emitDesktopStateChanged("backend-stopped")
	})
	controlMenu.AddSeparator()
	controlMenu.AddText("启动开发环境", nil, func(*menu.CallbackData) {
		_ = a.startDeveloperEnvironmentFromSettings()
		a.emitDesktopStateChanged("developer-started")
	})
	controlMenu.AddText("停止开发环境", nil, func(*menu.CallbackData) {
		_ = a.StopDeveloperEnvironment()
		a.emitDesktopStateChanged("developer-stopped")
	})
	controlMenu.AddText("启动开发隧道", nil, func(*menu.CallbackData) {
		_ = a.startDeveloperTunnelFromSettings()
		a.emitDesktopStateChanged("developer-tunnel-started")
	})
	controlMenu.AddText("停止开发隧道", nil, func(*menu.CallbackData) {
		_ = a.stopDeveloperTunnel()
		a.emitDesktopStateChanged("developer-tunnel-stopped")
	})
	controlMenu.AddSeparator()
	controlMenu.AddText("退出 OpenWatcher", nil, func(*menu.CallbackData) {
		a.quitApplication()
	})

	items := []*menu.MenuItem{
		menu.SubMenu("控制", controlMenu),
	}
	if runtime.GOOS == "darwin" {
		items = append([]*menu.MenuItem{menu.AppMenu()}, items...)
		items = append(items, menu.EditMenu(), menu.WindowMenu())
	}
	return menu.NewMenuFromItems(items[0], items[1:]...)
}

func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Show(a.ctx)
	wailsruntime.WindowShow(a.ctx)
}

func (a *App) quitApplication() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Quit(a.ctx)
}

func (a *App) startDeveloperEnvironmentFromSettings() DeveloperEnvironmentSnapshot {
	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		loaded = settings.DefaultDesktopSettings()
	}
	request := developerRequestFromSettings(loaded.DeveloperEnvironment, true)
	return a.EnsureDeveloperEnvironment(request)
}

func (a *App) startDeveloperTunnelFromSettings() DeveloperEnvironmentSnapshot {
	loaded, err := settings.LoadDesktopSettings()
	if err != nil {
		loaded = settings.DefaultDesktopSettings()
	}
	request := developerRequestFromSettings(loaded.DeveloperEnvironment, true)
	request.ManagedTunnelEnabled = true
	return a.EnsureDeveloperEnvironment(request)
}

func (a *App) stopDeveloperTunnel() DeveloperEnvironmentSnapshot {
	if a.devTunnelManager != nil {
		_ = a.devTunnelManager.Stop(context.Background())
	}
	status := devenv.Status{}
	if a.devEnvManager != nil {
		status = a.devEnvManager.Status()
	}
	request := developerRequestFromStatus(status, status.Running)
	request.ManagedTunnelEnabled = false
	_ = a.persistDeveloperEnvironmentPreference(request, status.Running)
	return a.developerEnvironmentSnapshot(status)
}

func (a *App) emitDesktopStateChanged(source string) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "desktop-state-changed", source)
}
