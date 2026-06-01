package main

import (
	"embed"
	_ "embed"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIcon []byte

func main() {
	store, err := NewStore("clipboard.db")
	if err != nil {
		log.Fatal(err)
	}

	clipboardService := NewClipboardService(store)

	app := application.New(application.Options{
		Name:        "Clipboard Manager",
		Description: "A clipboard history manager",
		LogLevel:    slog.LevelDebug,
		Services: []application.Service{
			application.NewService(clipboardService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// Keep running when window is closed
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		KeyBindings: map[string]func(window application.Window){
			"CmdOrCtrl+Shift+V": func(window application.Window) {
				if window.IsVisible() {
					window.Hide()
				} else {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	// Create main window
	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Clipboard Manager",
		Width:  420,
		Height: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
		},
	})

	// Hook window close: hide instead of closing
	mainWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		mainWindow.Hide()
		event.Cancel()
	})

	// System tray
	tray := app.SystemTray.New()
	tray.SetTemplateIcon(trayIcon)
	tray.SetTooltip("Clipboard Manager")

	// Tray menu (right-click)
	trayMenu := app.NewMenu()
	trayMenu.Add("显示窗口").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("退出").OnClick(func(ctx *application.Context) {
		app.Quit()
	})
	tray.SetMenu(trayMenu)

	// Click tray icon to toggle window
	tray.AttachWindow(mainWindow).WindowOffset(4)

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
