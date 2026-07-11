//go:build windows

package main

import "context"

// Wails owns the HWND; release host integration can set tool-window/DPI here.
func setWidgetPlatform(context.Context) {}
