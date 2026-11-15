package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"log"
	goruntime "runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	m := menu.NewMenu()
	if goruntime.GOOS == "darwin" {
		m.Append(menu.AppMenu())
		m.Append(menu.EditMenu())
	}
	if err := wails.Run(&options.App{
		Title: "Bajiu DB · 捌玖", Width: 1240, Height: 820, MinWidth: 960, MinHeight: 680,
		BackgroundColour: options.NewRGB(245, 247, 250),
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app}, Menu: m,
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "b20cd936-a701-4eed-95d0-82cd44372f09", OnSecondInstanceLaunch: func(_ options.SecondInstanceData) { runtime.WindowUnminimise(app.ctx); runtime.Show(app.ctx) }},
		Windows:            &windows.Options{Theme: windows.Light, DisablePinchZoom: true},
	}); err != nil {
		log.Print("Bajiu DB 无法启动: ", err)
	}
}
