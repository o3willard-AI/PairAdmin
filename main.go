package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pairadmin/services"
	"pairadmin/services/capture"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create CommandService for clipboard and Wayland detection
	commands := services.NewCommandService()

	// Create LLMService with config from environment variables
	llmService := services.NewLLMService(services.LoadConfig())

	// Create CaptureManager with TmuxAdapter and ATSPIAdapter for terminal discovery and capture
	tmuxAdapter := capture.NewTmuxAdapter()
	atspiAdapter := capture.NewATSPIAdapter()
	manager := capture.NewCaptureManager([]capture.TerminalAdapter{tmuxAdapter, atspiAdapter}, runtime.EventsEmit)

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "PairAdmin",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 255},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			commands.Startup(ctx)
			llmService.Startup(ctx)
			manager.Startup(ctx)
		},
		Bind: []interface{}{
			app,
			commands,
			llmService,
			manager,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
