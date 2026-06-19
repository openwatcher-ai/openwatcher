//go:build darwin && cgo

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void OpenWatcherInstallStatusItem(void);
void OpenWatcherRefreshStatusItem(int serviceRunning, int developerRunning, int developerTunnelRunning);
*/
import "C"

import "sync"

var statusItemApp struct {
	sync.RWMutex
	app *App
}

func (a *App) installStatusItem() {
	statusItemApp.Lock()
	statusItemApp.app = a
	statusItemApp.Unlock()
	C.OpenWatcherInstallStatusItem()
}

func (a *App) refreshStatusItem() {
	if a == nil {
		return
	}
	serviceRunning := false
	if a.backendManager != nil {
		serviceRunning = a.backendManager.DesktopStatus().Running
	}
	developerRunning := false
	if a.devEnvManager != nil {
		developerRunning = a.devEnvManager.Status().Running
	}
	developerTunnelRunning := false
	if a.devTunnelManager != nil {
		developerTunnelRunning = a.devTunnelManager.Status().Running
	}
	C.OpenWatcherRefreshStatusItem(cBool(serviceRunning), cBool(developerRunning), cBool(developerTunnelRunning))
}

func currentStatusItemApp() *App {
	statusItemApp.RLock()
	defer statusItemApp.RUnlock()
	return statusItemApp.app
}

func runStatusItemAction(action func(*App)) {
	app := currentStatusItemApp()
	if app == nil {
		return
	}
	go action(app)
}

func cBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

//export openwatcherStatusShow
func openwatcherStatusShow() {
	runStatusItemAction(func(app *App) {
		app.showMainWindow()
	})
}

//export openwatcherStatusStartBackend
func openwatcherStatusStartBackend() {
	runStatusItemAction(func(app *App) {
		_ = app.StartBackend()
		app.emitDesktopStateChanged("status-item-backend-started")
	})
}

//export openwatcherStatusStopBackend
func openwatcherStatusStopBackend() {
	runStatusItemAction(func(app *App) {
		_ = app.StopBackend()
		app.emitDesktopStateChanged("status-item-backend-stopped")
	})
}

//export openwatcherStatusStartDeveloper
func openwatcherStatusStartDeveloper() {
	runStatusItemAction(func(app *App) {
		_ = app.startDeveloperEnvironmentFromSettings()
		app.emitDesktopStateChanged("status-item-developer-started")
	})
}

//export openwatcherStatusStopDeveloper
func openwatcherStatusStopDeveloper() {
	runStatusItemAction(func(app *App) {
		_ = app.StopDeveloperEnvironment()
		app.emitDesktopStateChanged("status-item-developer-stopped")
	})
}

//export openwatcherStatusStartDeveloperTunnel
func openwatcherStatusStartDeveloperTunnel() {
	runStatusItemAction(func(app *App) {
		_ = app.startDeveloperTunnelFromSettings()
		app.emitDesktopStateChanged("status-item-developer-tunnel-started")
	})
}

//export openwatcherStatusStopDeveloperTunnel
func openwatcherStatusStopDeveloperTunnel() {
	runStatusItemAction(func(app *App) {
		_ = app.stopDeveloperTunnel()
		app.emitDesktopStateChanged("status-item-developer-tunnel-stopped")
	})
}

//export openwatcherStatusRefresh
func openwatcherStatusRefresh() {
	app := currentStatusItemApp()
	if app == nil {
		return
	}
	app.refreshStatusItem()
}

//export openwatcherStatusQuit
func openwatcherStatusQuit() {
	runStatusItemAction(func(app *App) {
		app.quitApplication()
	})
}
