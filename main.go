package main

import (
	"embed"
	_ "embed"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/trayicon.png
var trayIcon []byte

func main() {
	// Use ~/Library/Application Support/ClipboardManager for data
	dataDir := getDataDir()
	dbPath := filepath.Join(dataDir, "clipboard.db")

	store, err := NewStore(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	clipboardService := NewClipboardService(store)

	app := application.New(application.Options{
		Name:        "Clipboard Manager",
		Description: "A clipboard history manager",
		LogLevel:    slog.LevelDebug,
		Services: []application.Service{
			application.NewServiceWithOptions(clipboardService, application.ServiceOptions{
				Route: "/api/clipboard",
			}),
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

func getDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	var dir string
	switch {
	case filepath.Separator == '\\': // Windows
		appData := os.Getenv("APPDATA")
		if appData != "" {
			dir = filepath.Join(appData, "ClipboardManager")
		} else {
			dir = filepath.Join(home, "AppData", "Roaming", "ClipboardManager")
		}
	default: // macOS / Linux
		dir = filepath.Join(home, "Library", "Application Support", "ClipboardManager")
		if _, err := os.Stat(filepath.Join(home, "Library")); os.IsNotExist(err) {
			// Linux: use XDG data dir
			xdg := os.Getenv("XDG_DATA_HOME")
			if xdg == "" {
				xdg = filepath.Join(home, ".local", "share")
			}
			dir = filepath.Join(xdg, "clipboard-manager")
		}
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Fatal(err)
	}
	return dir
}
