package main

import (
	"context"
	"embed"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"openwatcher/desktop-app/widget/internal/widgetapi"
	"openwatcher/desktop-app/widget/internal/widgetprefs"
	"openwatcher/desktop-app/widget/internal/widgetvm"
	"openwatcher/desktop-app/widget/internal/widgetwindow"
)

type App struct {
	ctx    context.Context
	assets embed.FS
	mu     sync.RWMutex
	state  widgetvm.State
	client *widgetapi.Client
	prefs  widgetprefs.Store
	window widgetwindow.Geometry
	cancel context.CancelFunc
}

func NewApp(assets embed.FS) *App {
	return &App{
		assets: assets,
		state:  widgetvm.InitialState(),
		client: widgetapi.NewClient(widgetapi.DefaultEndpoint, widgetapi.NoTokenSource{}),
		prefs:  widgetprefs.NewMemoryStore(),
		window: widgetwindow.DefaultGeometry(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowSetAlwaysOnTop(ctx, true)
	runtime.WindowSetSize(ctx, 56, 56)
	setWidgetPlatform(ctx)
	a.mu.Lock()
	a.window = widgetwindow.Resize(a.window, false, 1920, 1080)
	a.mu.Unlock()
	a.startClient()
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.mu.Unlock()
}

func (a *App) startClient() {
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.cancel = cancel
	a.mu.Unlock()
	go a.client.Run(ctx, func(state widgetvm.State) {
		a.mu.Lock()
		a.state = state
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "widget:state", state)
	})
}

func (a *App) State() widgetvm.State { a.mu.RLock(); defer a.mu.RUnlock(); return a.state }

func (a *App) Refresh() { go a.client.Refresh(context.Background()) }

func (a *App) Toggle() {
	a.mu.Lock()
	a.state.Expanded = !a.state.Expanded
	expanded := a.state.Expanded
	a.mu.Unlock()
	if expanded {
		runtime.WindowSetSize(a.ctx, 1120, 580)
	} else {
		runtime.WindowSetSize(a.ctx, 56, 56)
	}
	runtime.EventsEmit(a.ctx, "widget:state", a.State())
}

func (a *App) Collapse() {
	a.mu.Lock()
	a.state.Expanded = false
	a.mu.Unlock()
	runtime.WindowSetSize(a.ctx, 56, 56)
	runtime.EventsEmit(a.ctx, "widget:state", a.State())
}

func (a *App) DragFinished(x, y, screenWidth, screenHeight float64) {
	a.mu.Lock()
	a.window = widgetwindow.Snap(widgetwindow.Point{X: x, Y: y}, widgetwindow.WorkArea{Width: screenWidth, Height: screenHeight}, 56, 8)
	a.mu.Unlock()
}

func (a *App) SetObservedNow() { a.mu.Lock(); a.state.LastActionAt = time.Now().UTC(); a.mu.Unlock() }
