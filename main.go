package main

import (
	"embed"
	"log"
	goruntime "runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/ISerranoDev/WailsCommander/internal/app"
	"github.com/ISerranoDev/WailsCommander/internal/config"
	"github.com/ISerranoDev/WailsCommander/internal/settings"
	"github.com/ISerranoDev/WailsCommander/internal/vault"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	vaultPath, err := config.VaultPath()
	if err != nil {
		log.Fatalf("resolve data directory: %v", err)
	}
	settingsPath, err := config.SettingsPath()
	if err != nil {
		log.Fatalf("resolve data directory: %v", err)
	}
	desktopApp := app.New(vault.New(vaultPath), settings.NewStore(settingsPath))

	err = wails.Run(&options.App{
		Title:     config.AppName,
		Width:     1120,
		Height:    740,
		MinWidth:  760,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 18, B: 23, A: 255},
		Mac: &mac.Options{
			// Content runs under the traffic lights, like Termius.
			TitleBar: mac.TitleBarHiddenInset(),
		},
		Windows: &windows.Options{
			// Dark native title bar on Windows 10/11 to match the UI.
			Theme: windows.Dark,
		},
		Linux: &linux.Options{
			Icon:        icon,
			ProgramName: config.AppName,
		},
		OnStartup:  desktopApp.Startup,
		OnShutdown: desktopApp.Shutdown,
		Bind: []interface{}{
			desktopApp,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// appMenu is needed on macOS: without an Edit menu the webview ignores
// Cmd+C / Cmd+V / Cmd+A, both in inputs and in the terminal.
func appMenu() *menu.Menu {
	if goruntime.GOOS != "darwin" {
		return nil
	}
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
	m.Append(menu.EditMenu())
	m.Append(menu.WindowMenu())
	return m
}
