package main

import (
	"context"
	"embed"
	"log"

	"github.com/clip-rss/clip/api"
	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// wailsEmitter 用 Wails 运行时事件总线实现 scheduler.Emitter，
// 通过全局 application.Get() 解耦，避免与 App 实例的构造顺序耦合。
type wailsEmitter struct{}

func (wailsEmitter) Emit(name string, data any) {
	if app := application.Get(); app != nil {
		app.Event.Emit(name, data)
	}
}

func main() {
	// 数据层。
	st, err := store.New()
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	// 抓取与调度层。
	ft := fetcher.New()
	sch := scheduler.New(st, ft, scheduler.WithEmitter(wailsEmitter{}))

	// 绑定服务（暴露给前端）。
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
			application.NewService(api.NewFeedService(st, ft, sch)),
			application.NewService(api.NewItemService(st)),
			application.NewService(api.NewCategoryService(st)),
			application.NewService(api.NewSettingsService(st)),
			application.NewService(api.NewOPMLService(st)),
		},
	})

	// 启动后台调度（此时 application.Get() 已可用于事件推送）。
	sch.Start(context.Background())

	// 退出时优雅停机：先停调度，再关数据库。
	app.OnShutdown(func() {
		sch.Stop()
		_ = st.Close()
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

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
