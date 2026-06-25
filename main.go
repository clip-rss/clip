package main

import (
	"context"
	"embed"
	"log"
	"strconv"
	"strings"
	"time"

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

const (
	defaultWindowWidth  = 1200
	defaultWindowHeight = 800
	minWindowWidth      = 800
	minWindowHeight     = 600
)

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

func savedWindowSize(settings store.Settings) (int, int) {
	width := settings.WindowWidth
	height := settings.WindowHeight
	if width < minWindowWidth {
		width = defaultWindowWidth
	}
	if height < minWindowHeight {
		height = defaultWindowHeight
	}
	return width, height
}

func saveWindowSize(st *store.Store, window application.Window) {
	if window == nil {
		return
	}
	width, height := window.Size()
	if width < minWindowWidth || height < minWindowHeight {
		return
	}
	settings, err := st.GetSettings()
	if err != nil {
		log.Printf("failed to load settings before saving window size: %v", err)
		return
	}
	settings.WindowWidth = width
	settings.WindowHeight = height
	if err := st.UpdateSettings(settings); err != nil {
		log.Printf("failed to save window size: %v", err)
	}
}

func main() {
	// 数据层。
	st, err := store.New()
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	// 读取设置以决定窗口启动行为（最小化等）。读取失败时退回默认值。
	settings, _ := st.GetSettings()

	// 通知服务（需同时注册为 application.Service 并注入调度器）。
	notifSvc := notifications.New()
	notifSender := wailsNotifSender{ns: notifSvc}
	notifier := notify.NewService(st, notifSender)

	// 抓取与调度层。
	ft := fetcher.New()
	if settings.ProxyHost != "" && settings.ProxyPort > 0 {
		ft.Client().SetProxy(settings.ProxyHost, settings.ProxyPort)
	}
	sch := scheduler.New(st, ft,
		scheduler.WithEmitter(wailsEmitter{}),
		scheduler.WithNotifier(notifier),
		scheduler.WithConfig(scheduler.Config{
			DefaultInterval: time.Duration(settings.DefaultUpdateInterval) * time.Minute,
		}),
	)

	// 绑定服务（暴露给前端）。
	sysSvc := &api.SystemService{}
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
			application.NewService(sysSvc),
			application.NewService(api.NewFeedService(st, ft, sch)),
			application.NewService(api.NewItemService(st)),
			application.NewService(api.NewCategoryService(st)),
			application.NewService(api.NewSettingsService(st, sch, ft.Client())),
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

	// 启动行为：用户开启「启动时最小化」时以最小化状态创建窗口（保留任务栏/Dock 图标）。
	startState := application.WindowStateNormal
	if settings.LaunchMinimized {
		startState = application.WindowStateMinimised
	}
	windowWidth, windowHeight := savedWindowSize(settings)

	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:      "Clip",
		Width:      windowWidth,
		Height:     windowHeight,
		MinWidth:   minWindowWidth,
		MinHeight:  minWindowHeight,
		StartState: startState,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropTranslucent,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})
	mainWindow.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		saveWindowSize(st, mainWindow)
	})

	sysSvc.Window = mainWindow

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

