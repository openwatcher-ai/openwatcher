package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend-vue3/dist
var assets embed.FS

func main() {
	app := NewApp(assets)
	err := wails.Run(&options.App{
		Title:             "OpenWatcher Widget",
		Width:             1120,
		Height:            580,
		MinWidth:          920,
		MinHeight:         500,
		Frameless:         true,
		AlwaysOnTop:       true,
		DisableResize:     true,
		HideWindowOnClose: true,
		Assets:            assets,
		Bind:              []interface{}{app},
		OnStartup:         app.Startup,
		OnShutdown:        app.Shutdown,
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		Windows: &windows.Options{WebviewIsTransparent: true, WindowIsTranslucent: true},
	})
	if err != nil {
		log.Fatal(err)
	}
}
