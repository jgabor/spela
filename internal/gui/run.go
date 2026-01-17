package gui

import (
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func Run() error {
	app := NewApp()

	appMenu := menu.NewMenu()
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Scan games", keys.CmdOrCtrl("r"), func(cd *menu.CallbackData) {
		_ = app.ScanGames()
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(cd *menu.CallbackData) {
		runtime.Quit(app.ctx)
	})

	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Toggle fullscreen", keys.Key("F11"), func(cd *menu.CallbackData) {
		runtime.WindowToggleMaximise(app.ctx)
	})

	assetOpts := &assetserver.Options{
		Assets:  getAssets(),
		Handler: getDevHandler(),
	}

	return wails.Run(&options.App{
		Title:            "Spela",
		Width:            1024,
		Height:           768,
		AssetServer:      assetOpts,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Menu:             appMenu,
		Bind: []any{
			app,
		},
	})
}
