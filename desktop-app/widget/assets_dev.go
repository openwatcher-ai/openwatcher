//go:build !production

package main

import "embed"

// widgetAssets is deliberately stable: clean Go checkouts do not need ignored Vite output.
//
//go:embed all:frontend-fallback
var widgetAssets embed.FS

const assetRoot = "frontend-fallback"
