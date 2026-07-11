package main

import (
	"flag"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"openwatcher/desktop-app/widget/internal/widgetapi"
)

func main() {
	endpoint := flag.String("endpoint", "", "loopback OpenWatcher Widget endpoint")
	flag.Parse()
	if !widgetapi.ValidEndpoint(*endpoint) {
		log.Fatal("invalid widget endpoint")
	}
	assets, err := fs.Sub(widgetAssets, assetRoot)
	if err != nil {
		log.Fatal(err)
	}
	app := NewApp(AppDependencies{Endpoint: *endpoint, TokenSource: widgetapi.NoTokenSource{}})
	err = wails.Run(&options.App{
		Title:              "OpenWatcher Widget",
		Width:              56,
		Height:             56,
		MinWidth:           56,
		MinHeight:          56,
		MaxWidth:           1120,
		MaxHeight:          580,
		Frameless:          true,
		AlwaysOnTop:        true,
		DisableResize:      true,
		HideWindowOnClose:  true,
		BackgroundColour:   options.NewRGBA(0, 0, 0, 0),
		AssetServer:        &assetserver.Options{Assets: assets},
		Bind:               []interface{}{app},
		OnStartup:          app.Startup,
		OnShutdown:         app.Shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "ai.openwatcher.widget"},
		Mac:                &mac.Options{WebviewIsTransparent: true, WindowIsTranslucent: true, DisableZoom: true},
		Windows:            &windows.Options{WebviewIsTransparent: true, WindowIsTranslucent: true, DisableWindowIcon: true},
	})
	if err != nil {
		log.Fatal(err)
	}
}
