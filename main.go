package main

import (
	"context"
	"embed"
	"os"
	"path/filepath"

	"github.com/awnumar/memguard"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pairadmin/services"
	"pairadmin/services/audit"
	"pairadmin/services/capture"
	"pairadmin/services/keychain"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Full software-rendering stack for systems without reliable GPU support
	// (VMs with VirtIO/Virtio GPU, no-dGPU desktops, headless).
	// All vars must be set before wails.Run() so GTK/WebKit pick them up at init.
	//
	// LIBGL_ALWAYS_SOFTWARE: Mesa bypasses the Zink/Vulkan path that hangs on no-GPU systems.
	// WEBKIT_DISABLE_COMPOSITING_MODE: stops WebKit GPU compositing.
	// WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1: WebKit ≥2.40 sandbox rename; the sandbox
	//   is blocked by seccomp/namespace restrictions on many Linux configs (blank window).
	// WEBKIT_DISABLE_DMABUF_RENDERER=1: DMA-BUF renderer silently fails on VirtIO/Intel/AMD
	//   without full kernel DRM support.
	// GDK_BACKEND=x11 and GSK_RENDERER=cairo are intentionally NOT set: forcing X11 on
	// a native-Wayland session prevents the brief blink of content that proves rendering
	// works on the Wayland path; xWayland compositing is worse, not better, on VirtIO.
	os.Setenv("LIBGL_ALWAYS_SOFTWARE", "1")
	os.Setenv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")

	// CatchInterrupt registers a signal handler so that memguard Enclaves are purged on SIGINT/SIGTERM.
	// Must be called before any Enclave creation.
	memguard.CatchInterrupt()

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

	// Create PTYService for interactive shell sessions in + New terminal tabs
	ptyService := services.NewPTYService()

	// Create SettingsService with OS keychain for secure API key storage
	keychainClient := keychain.New()
	settingsService := services.NewSettingsService(keychainClient)
	settingsService.SetLLMService(llmService)
	settingsService.SetCaptureManager(manager)

	// Load API keys from keychain and seal into memguard Enclaves.
	providers := []string{"openai", "anthropic", "openrouter"}
	for _, p := range providers {
		rawKey, err := keychainClient.Get(p)
		if err == nil && rawKey != "" {
			buf := memguard.NewBufferFromBytes([]byte(rawKey))
			llmService.SetAPIKeyEnclave(p, buf.Seal())
		}
	}
	// Rebuild provider now that Enclaves are loaded.
	llmService.RebuildProvider()

	// Declare sessionID and auditLogger in main() scope so both OnStartup and OnBeforeClose closures can reference them.
	var sessionID string
	var auditLogger *audit.AuditLogger

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
			// Generate session UUID and create audit logger.
			sessionID = uuid.New().String()
			home, _ := os.UserHomeDir()
			auditLogger, _ = audit.NewAuditLogger(filepath.Join(home, ".pairadmin", "logs"))

			// Inject audit logger into services.
			llmService.SetAuditLogger(auditLogger, sessionID)
			commands.SetAuditLogger(auditLogger, sessionID)

			// Write session_start audit entry.
			if auditLogger != nil {
				auditLogger.Write(audit.AuditEntry{Event: "session_start", SessionID: sessionID})
			}

			app.startup(ctx)
			commands.Startup(ctx)
			llmService.Startup(ctx)
			manager.Startup(ctx)
			settingsService.Startup(ctx)
			ptyService.Startup(ctx)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			if auditLogger != nil {
				auditLogger.Write(audit.AuditEntry{Event: "session_end", SessionID: sessionID})
			}
			memguard.Purge()
			return false
		},
		Bind: []interface{}{
			app,
			commands,
			llmService,
			manager,
			settingsService,
			ptyService,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
