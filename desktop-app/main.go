package main

import (
	"embed"
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend-vue3
var assets embed.FS

func main() {
	app := NewApp()
	frontendAssets := fs.FS(assets)
	if distAssets, err := fs.Sub(assets, "frontend-vue3/dist"); err == nil {
		frontendAssets = distAssets
	}

	err := wails.Run(&options.App{
		Title:             "OpenWatcher",
		Width:             1320,
		Height:            860,
		MinWidth:          1120,
		MinHeight:         760,
		HideWindowOnClose: true,
		BackgroundColour:  &options.RGBA{R: 10, G: 17, B: 26, A: 1},
		AssetServer: &assetserver.Options{
			Assets: frontendAssets,
		},
		Menu: app.applicationMenu(),
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("OpenWatcher 启动失败:", err.Error())
	}
}
