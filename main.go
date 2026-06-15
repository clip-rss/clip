package main

import (
	"embed"
	"log"

	"changeme/api"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "clip",
		Description: "跨平台 RSS 阅读器",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(&api.SystemService{}),
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Clip",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 48,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
