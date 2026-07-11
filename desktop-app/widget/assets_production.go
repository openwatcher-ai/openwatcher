//go:build production

package main

import "embed"

// Production Wails builds pass the production tag after Vite has created dist.
//
//go:embed all:frontend-vue3/dist
var widgetAssets embed.FS

const assetRoot = "frontend-vue3/dist"
