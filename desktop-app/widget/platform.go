package main

import (
	"context"

	"openwatcher/desktop-app/widget/internal/widgetwindow"
)

type nativePlatform struct{}

func (nativePlatform) WorkAreas(context.Context) []widgetwindow.WorkArea { return nativeWorkAreas() }
func (nativePlatform) WindowGeometry(ctx context.Context) widgetwindow.Geometry {
	return nativeWindowGeometry(ctx)
}
func (nativePlatform) ApplyGeometry(ctx context.Context, geometry widgetwindow.Geometry) {
	nativeApplyGeometry(ctx, geometry)
}
func (nativePlatform) OpenMainApp(context.Context) { nativeOpenMainApp() }
