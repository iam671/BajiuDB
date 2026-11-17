package main

import (
	"embed"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte
var instanceID = "b20cd936-a701-4eed-95d0-82cd44372f09"
var windowTrace = ""

func main() {
	app := NewApp()
	app.desktop = application.New(application.Options{Name: "Bajiu DB", Icon: appIcon, Description: "Bajiu DB · 捌玖", Services: []application.Service{application.NewService(app)}, Assets: application.AssetOptions{Handler: application.BundledAssetFileServer(assets)}, OnShutdown: app.shutdown, SingleInstance: &application.SingleInstanceOptions{UniqueID: instanceID, OnSecondInstanceLaunch: func(application.SecondInstanceData) {
		if app.mainWindow != nil {
			app.mainWindow.Restore()
			app.updateModalWindows()
		}
	}}})
	app.mainWindow = app.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: "main", Title: "Bajiu DB · 捌玖", Width: 1240, Height: 820, MinWidth: 960, MinHeight: 680, URL: "/?desktop=1", BackgroundColour: application.NewRGB(245, 247, 250), Windows: application.WindowsWindow{Theme: application.Light}})
	app.mainWindow.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { app.desktop.Quit() })
	if windowTrace == "1" {
		for _, event := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize, events.Common.WindowRuntimeReady} {
			app.mainWindow.OnWindowEvent(event, func(*application.WindowEvent) { app.traceWindows() })
		}
	}
	if err := app.desktop.Run(); err != nil {
		log.Print("Bajiu DB 无法启动: ", err)
	}
}
