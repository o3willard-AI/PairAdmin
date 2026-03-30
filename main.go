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
	"pairadmin/services/keychain"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create CommandService for clipboard and Wayland detection
	commands := services.NewCommandService()

	// Create LLMService using Viper-first config (D-04: Viper > env var priority)
	llmService := services.NewLLMService(services.LoadConfigWithViper())

	// Create CaptureManager with TmuxAdapter and ATSPIAdapter for terminal discovery and capture
	tmuxAdapter := capture.NewTmuxAdapter()
	atspiAdapter := capture.NewATSPIAdapter()
	manager := capture.NewCaptureManager([]capture.TerminalAdapter{tmuxAdapter, atspiAdapter}, runtime.EventsEmit)

	// Wire CaptureManager to LLMService so FilterCommand can trigger pipeline rebuilds
	llmService.SetCaptureManager(manager)

	// Create SettingsService with OS keychain for secure API key storage
	keychainClient := keychain.New()
	settingsService := services.NewSettingsService(keychainClient)
	settingsService.SetLLMService(llmService)
	settingsService.SetCaptureManager(manager)

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
			settingsService.Startup(ctx)
		},
		Bind: []interface{}{
			app,
			commands,
			llmService,
			manager,
			settingsService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
