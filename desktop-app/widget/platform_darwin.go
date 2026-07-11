//go:build darwin

package main

import "context"

// Wails v2.12 exposes transparency and frameless options but not activation
// policy or visibleFrame. Keep this seam for the native host integration.
func setWidgetPlatform(context.Context) {}
