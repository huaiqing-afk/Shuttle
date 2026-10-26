package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// MinWidth/MinHeight fit a 1080x720 screen at 150% scaling (720x480 DIP);
// the next common step down would crop the window off-screen on such displays.
func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title: "Shuttle · 接驳", Width: 1280, Height: 820, MinWidth: 720, MinHeight: 480,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup, OnShutdown: app.shutdown, OnBeforeClose: app.beforeClose,
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
