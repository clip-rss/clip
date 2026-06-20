package main

import (
	"context"
	"embed"
	"log"
	"strconv"
	"strings"

	"github.com/clip-rss/clip/api"
	"github.com/clip-rss/clip/internal/fetcher"
	"github.com/clip-rss/clip/internal/notify"
	"github.com/clip-rss/clip/internal/scheduler"
	"github.com/clip-rss/clip/internal/store"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
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

// wailsNotifSender 用 Wails notifications 服务实现 notify.Sender。
type wailsNotifSender struct {
	ns *notifications.NotificationService
}

func (s wailsNotifSender) Send(msg notify.Message) error {
	return s.ns.SendNotification(notifications.NotificationOptions{
		ID:    msg.ID,
		Title: msg.Title,
		Body:  msg.Body,
		Data:  map[string]interface{}{"articleId": msg.ID},
	})
}

func main() {
	// 数据层。
	st, err := store.New()
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	// 通知服务（需同时注册为 application.Service 并注入调度器）。
	notifSvc := notifications.New()
	notifSender := wailsNotifSender{ns: notifSvc}
	notifier := notify.NewService(st, notifSender)

	// 抓取与调度层。
	ft := fetcher.New()
	sch := scheduler.New(st, ft,
		scheduler.WithEmitter(wailsEmitter{}),
		scheduler.WithNotifier(notifier),
	)

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
			application.NewService(notifSvc),
		},
	})

	// 点击通知 → 调起窗口 + 向前端推送 article ID，前端自行定位。
	var mainWindow application.Window
	notifSvc.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Error != nil {
			log.Printf("notification response error: %v", result.Error)
			return
		}
		rawID, _ := result.Response.UserInfo["articleId"].(string)
		if rawID == "" {
			return
		}
		idStr := strings.TrimPrefix(rawID, "article:")
		articleID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		if mainWindow != nil {
			mainWindow.UnMinimise()
			mainWindow.Restore()
			mainWindow.Focus()
		}
		app.Event.Emit("notification:open", map[string]any{"articleId": articleID})
	})

	// macOS：应用启动完毕后请求通知权限（用户授权后通知才能弹出）。
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(event *application.ApplicationEvent) {
		notifSvc.RequestNotificationAuthorization()
	})

	// 启动后台调度（此时 application.Get() 已可用于事件推送）。
	sch.Start(context.Background())

	// 退出时优雅停机：先停调度，再关数据库。
	app.OnShutdown(func() {
		sch.Stop()
		_ = st.Close()
	})

	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Clip",
		Width:     1200,
		Height:    800,
		MinWidth:  800,
		MinHeight: 600,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

