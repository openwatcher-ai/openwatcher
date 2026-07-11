package main

import (
	"context"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"openwatcher/desktop-app/widget/internal/widgetapi"
	"openwatcher/desktop-app/widget/internal/widgetprefs"
	"openwatcher/desktop-app/widget/internal/widgetvm"
	"openwatcher/desktop-app/widget/internal/widgetwindow"
)

type AppDependencies struct {
	Endpoint    string
	TokenSource widgetapi.TokenSource
	Prefs       widgetprefs.Store
	TrendStore  widgetapi.TrendStore
	Platform    Platform
}
type Platform interface {
	WorkAreas(context.Context) []widgetwindow.WorkArea
	WindowGeometry(context.Context) widgetwindow.Geometry
	ApplyGeometry(context.Context, widgetwindow.Geometry)
	OpenMainApp(context.Context)
}
type App struct {
	ctx      context.Context
	mu       sync.RWMutex
	state    widgetvm.State
	client   *widgetapi.Client
	prefs    widgetprefs.Store
	window   widgetwindow.Geometry
	platform Platform
	cancel   context.CancelFunc
}

func NewApp(deps AppDependencies) *App {
	if deps.TokenSource == nil {
		deps.TokenSource = widgetapi.NoTokenSource{}
	}
	if deps.Prefs == nil {
		deps.Prefs = widgetprefs.NewFileStore(widgetprefs.DefaultPath(userHome()))
	}
	if deps.TrendStore == nil {
		deps.TrendStore = widgetprefs.NewTrendFileStore(widgetprefs.DefaultTrendPath(userHome()))
	}
	if deps.Platform == nil {
		deps.Platform = nativePlatform{}
	}
	return &App{state: widgetvm.InitialState(), client: widgetapi.NewClient(deps.Endpoint, deps.TokenSource, deps.TrendStore), prefs: deps.Prefs, platform: deps.Platform, window: widgetwindow.DefaultGeometry()}
}
func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	setWidgetPlatform(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	a.mu.Lock()
	a.window = widgetwindow.Restore(toSaved(a.prefs.Load()), a.areas())
	a.state.Expanded = false
	a.state.AnchorCorner = widgetwindow.AnchorCorner(a.window, areaForGeometry(a.window, a.areas()))
	g := a.window
	a.mu.Unlock()
	a.apply(g)
	a.startClient()
}
func (a *App) Shutdown(context.Context) {
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
	go a.client.Run(ctx, func(s widgetvm.State) {
		a.mu.Lock()
		s.Expanded = a.state.Expanded
		s.AnchorCorner = a.state.AnchorCorner
		a.state = s
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "widget:state", s)
	})
}
func (a *App) State() widgetvm.State { a.mu.RLock(); defer a.mu.RUnlock(); return a.state }

// Refresh returns the current state so generated Wails JS has a stable Promise<State> contract.
func (a *App) Refresh() widgetvm.State { a.client.Refresh(); return a.State() }
func (a *App) Toggle() widgetvm.State {
	a.mu.RLock()
	expanded := a.state.Expanded
	a.mu.RUnlock()
	if expanded {
		return a.Collapse()
	}
	return a.Expand()
}
func (a *App) Expand() widgetvm.State {
	current := a.platform.WindowGeometry(a.ctx)
	area := areaForGeometry(current, a.areas())
	anchor := current
	corner := widgetwindow.AnchorCorner(anchor, area)
	a.mu.Lock()
	a.state.Expanded = true
	a.state.AnchorCorner = corner
	a.window = widgetwindow.ExpandAt(anchor, area, corner)
	g := a.window
	s := a.state
	a.mu.Unlock()
	a.apply(g)
	runtime.EventsEmit(a.ctx, "widget:state", s)
	return s
}
func (a *App) Collapse() widgetvm.State {
	anchor := widgetwindow.Restore(toSaved(a.prefs.Load()), a.areas())
	area := areaForGeometry(anchor, a.areas())
	a.mu.Lock()
	a.state.Expanded = false
	a.state.AnchorCorner = widgetwindow.AnchorCorner(anchor, area)
	a.window = anchor
	g := a.window
	s := a.state
	a.mu.Unlock()
	a.apply(g)
	runtime.EventsEmit(a.ctx, "widget:state", s)
	return s
}

// SnapCurrentWindow is invoked after the draggable orb pointer-up; coordinates come from native window position.
func (a *App) SnapCurrentWindow() widgetvm.State {
	window := a.platform.WindowGeometry(a.ctx)
	areas := a.areas()
	a.mu.Lock()
	anchor := window
	if a.state.Expanded {
		anchor = widgetwindow.AnchorFromPanel(window, a.state.AnchorCorner)
	}
	area := areaForGeometry(anchor, areas)
	anchor = widgetwindow.Snap(widgetwindow.Point{X: anchor.X, Y: anchor.Y}, area, widgetwindow.Orb, widgetwindow.Margin)
	_ = a.prefs.Save(fromSaved(widgetwindow.Save(anchor, area)))
	corner := widgetwindow.AnchorCorner(anchor, area)
	a.state.AnchorCorner = corner
	if a.state.Expanded {
		a.window = widgetwindow.ExpandAt(anchor, area, corner)
	} else {
		a.window = anchor
	}
	g := a.window
	s := a.state
	a.mu.Unlock()
	a.apply(g)
	return s
}
func (a *App) OpenMainApp() { a.platform.OpenMainApp(a.ctx) }
func (a *App) apply(g widgetwindow.Geometry) {
	a.platform.ApplyGeometry(a.ctx, g)
}
func (a *App) areas() []widgetwindow.WorkArea {
	if x := a.platform.WorkAreas(a.ctx); len(x) > 0 {
		return x
	}
	return []widgetwindow.WorkArea{{Width: 1920, Height: 1080, MonitorID: "primary"}}
}
func areaForGeometry(g widgetwindow.Geometry, areas []widgetwindow.WorkArea) widgetwindow.WorkArea {
	if len(areas) == 0 {
		return widgetwindow.WorkArea{Width: 1920, Height: 1080, MonitorID: "primary"}
	}
	x, y := g.X+g.Width/2, g.Y+g.Height/2
	for _, area := range areas {
		if x >= area.X && x < area.X+area.Width && y >= area.Y && y < area.Y+area.Height {
			return area
		}
	}
	return areas[0]
}
func toSaved(p widgetprefs.Position) widgetwindow.SavedPosition {
	return widgetwindow.SavedPosition{MonitorID: p.MonitorID, Edge: p.Edge, Normalized: p.Normalized}
}
func fromSaved(p widgetwindow.SavedPosition) widgetprefs.Position {
	return widgetprefs.Position{MonitorID: p.MonitorID, Edge: p.Edge, Normalized: p.Normalized}
}
