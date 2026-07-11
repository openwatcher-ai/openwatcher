//go:build !darwin && !windows

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"openwatcher/desktop-app/widget/internal/widgetwindow"
)

func setWidgetPlatform(context.Context) {}
func nativeWorkAreas() []widgetwindow.WorkArea {
	return []widgetwindow.WorkArea{{Width: 1920, Height: 1080, MonitorID: "primary"}}
}
func nativeWindowGeometry(ctx context.Context) widgetwindow.Geometry {
	x, y := runtime.WindowGetPosition(ctx)
	w, h := runtime.WindowGetSize(ctx)
	return widgetwindow.Geometry{X: float64(x), Y: float64(y), Width: float64(w), Height: float64(h)}
}
func nativeApplyGeometry(ctx context.Context, g widgetwindow.Geometry) {
	runtime.WindowSetSize(ctx, int(g.Width), int(g.Height))
	runtime.WindowSetPosition(ctx, int(g.X), int(g.Y))
}
func nativeOpenMainApp() {}
