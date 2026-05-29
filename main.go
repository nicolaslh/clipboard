package main

import (
	"embed"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

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
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		KeyBindings: map[string]func(window application.Window){
			"CmdOrCtrl+Shift+V": func(window application.Window) {
				// Toggle window visibility
				if window.IsVisible() {
					window.Hide()
				} else {
					window.Show()
					window.Focus()
				}
			},
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Clipboard Manager",
		Width:  420,
		Height: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
		},
	})

	err = app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
