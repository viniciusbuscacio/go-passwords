package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const appTitle = "go-passwords"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     appTitle,
		Width:     720,
		Height:    640,
		MinWidth:  480,
		MinHeight: 520,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Frameless:        true,
		StartHidden:      true, // shown by the frontend once the theme is applied
		BackgroundColour: &options.RGBA{R: 20, G: 24, B: 30, A: 0},
		OnStartup:        app.startup,
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.None,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		Linux: &linux.Options{
			WindowIsTranslucent: true,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			Icon:                appIcon,
			ProgramName:         "go-passwords",
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
