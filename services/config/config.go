package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// CustomPattern holds a user-defined filter pattern with a name, regex, and action.
type CustomPattern struct {
	Name   string `mapstructure:"name" yaml:"name"`
	Regex  string `mapstructure:"regex" yaml:"regex"`
	Action string `mapstructure:"action" yaml:"action"` // "redact" | "remove"
}

// PinnedCommand is a sidebar command explicitly saved (via the "Save Pinned"
// button) to survive across app restarts. The Commands sidebar itself is
// otherwise an in-memory-only, per-session list — this is the one subset a
// user has chosen to keep. Order in the slice is the pinned display order.
type PinnedCommand struct {
	Command          string `mapstructure:"command" yaml:"command"`
	OriginalQuestion string `mapstructure:"original_question" yaml:"original_question"`
}

// RemoteHost holds non-secret metadata for a saved remote terminal connection.
// Secrets (passwords, key passphrases) are never stored here — they live in the
// OS keychain, keyed by RemoteHost.ID (see services.RemoteService).
type RemoteHost struct {
	ID   string `mapstructure:"id" yaml:"id"`
	Kind string `mapstructure:"kind" yaml:"kind"` // "ssh" | "winrm"
	// Name is a user-assigned friendly label (e.g. via renaming the terminal
	// tab). Empty until the user renames it — display code should fall back
	// to "username@host" when empty rather than persisting that computed
	// form here, so a since-changed Host/Username still reflects correctly
	// for hosts that were never explicitly renamed.
	Name           string `mapstructure:"name" yaml:"name"`
	Host           string `mapstructure:"host" yaml:"host"`
	Port           int    `mapstructure:"port" yaml:"port"`
	Username       string `mapstructure:"username" yaml:"username"`
	AuthType       string `mapstructure:"auth_type" yaml:"auth_type"` // "password" | "privatekey"
	PrivateKeyPath string `mapstructure:"private_key_path" yaml:"private_key_path"`
	LastUsed       string `mapstructure:"last_used" yaml:"last_used"` // RFC3339
	// UseTmux/TmuxSessionName apply only to Kind == "ssh" — see
	// services.RemoteConnectParams for the create-or-attach behavior.
	UseTmux         bool   `mapstructure:"use_tmux" yaml:"use_tmux"`
	TmuxSessionName string `mapstructure:"tmux_session_name" yaml:"tmux_session_name"`
}

// AppConfig holds persistent application configuration (separate from the LLM env-var config).
type AppConfig struct {
	CustomPatterns     []CustomPattern `mapstructure:"custom_patterns" yaml:"custom_patterns"`
	Provider           string          `mapstructure:"provider" yaml:"provider"`
	Model              string          `mapstructure:"model" yaml:"model"`
	CustomPrompt       string          `mapstructure:"custom_prompt" yaml:"custom_prompt"`
	ATSPIPollingMs     int             `mapstructure:"atspi_polling_ms" yaml:"atspi_polling_ms"`
	ClipboardClearSecs int             `mapstructure:"clipboard_clear_secs" yaml:"clipboard_clear_secs"`
	HotkeyCopyLast     string          `mapstructure:"hotkey_copy_last" yaml:"hotkey_copy_last"`
	HotkeyFocusWindow  string          `mapstructure:"hotkey_focus_window" yaml:"hotkey_focus_window"`
	// HotkeyAddClipboardCommand grabs the current OS clipboard contents and
	// adds them as a new sidebar command — lets a user save a command they
	// derived directly in the terminal (select it, Ctrl+C, then this hotkey)
	// without going through the AI chat. Defaults to a combo not bound to
	// anything by default on Windows or macOS (see DefaultHotkeyAddClipboardCommand).
	HotkeyAddClipboardCommand string `mapstructure:"hotkey_add_clipboard_command" yaml:"hotkey_add_clipboard_command"`
	// HotkeyNewTerminal opens the "+ New" terminal dialog without requiring a
	// mouse trip to the bottom of the terminal list — see DefaultHotkeyNewTerminal.
	HotkeyNewTerminal string `mapstructure:"hotkey_new_terminal" yaml:"hotkey_new_terminal"`
	Theme              string          `mapstructure:"theme" yaml:"theme"`
	FontSize           int             `mapstructure:"font_size" yaml:"font_size"`
	ContextLines       int             `mapstructure:"context_lines" yaml:"context_lines"`
	OllamaHost         string          `mapstructure:"ollama_host" yaml:"ollama_host"`
	LMStudioHost       string          `mapstructure:"lmstudio_host" yaml:"lmstudio_host"`
	RemoteHosts        []RemoteHost    `mapstructure:"remote_hosts" yaml:"remote_hosts"`
	// TerminalsSidebarWidthCh/CommandsSidebarWidthCh size the left (terminal
	// list) and right (Quick Commands) sidebars, in CSS `ch` units (~1
	// character's width in the active UI font) rather than a fixed pixel
	// value, so a user on a much larger or smaller display can fit more or
	// fewer characters of a session/command name without the column becoming
	// a live-resizable (and bug-prone) drag handle. Takes effect on next
	// launch, not live — see DefaultTerminalsSidebarWidthCh /
	// DefaultCommandsSidebarWidthCh for the values approximating the
	// original hardcoded pixel widths.
	TerminalsSidebarWidthCh int `mapstructure:"terminals_sidebar_width_ch" yaml:"terminals_sidebar_width_ch"`
	CommandsSidebarWidthCh  int `mapstructure:"commands_sidebar_width_ch" yaml:"commands_sidebar_width_ch"`
	// PinnedCommands — see PinnedCommand's doc comment.
	PinnedCommands []PinnedCommand `mapstructure:"pinned_commands" yaml:"pinned_commands"`
	// PromptNewHostKeys controls SSH host key trust-on-first-use behavior.
	// When false (the default), the first time PairAdmin connects to a given
	// host:port it silently pins whatever key the server presents — most
	// target hosts don't give a user any independently-verifiable fingerprint
	// to check anyway, so a mandatory prompt on every new host would just
	// train users to click "accept" without looking. When true, that first
	// connection instead pauses for the user (or their security team) to
	// explicitly accept the presented fingerprint before it's pinned. Either
	// way, once a key IS pinned for a host:port, a later connection
	// presenting a DIFFERENT key is always rejected — that check is the
	// actual MITM defense and isn't affected by this setting.
	PromptNewHostKeys bool `mapstructure:"prompt_new_host_keys" yaml:"prompt_new_host_keys"`
}

// DefaultHotkeyAddClipboardCommand is the out-of-the-box binding for
// HotkeyAddClipboardCommand. Ctrl+Shift+<letter> avoids AltGr composition
// conflicts on European keyboard layouts (which Ctrl+Alt combos trigger),
// isn't a default Chromium/WebView2 devtools or browsing shortcut, and isn't
// claimed by a default macOS or Windows system-wide shortcut — safe to fire
// while the app window has focus on either platform.
const DefaultHotkeyAddClipboardCommand = "Ctrl+Shift+A"

// DefaultHotkeyNewTerminal is the out-of-the-box binding for
// HotkeyNewTerminal. WebView2/WebKit implement browser-shell features like
// tab/window/incognito management (which is where Ctrl+Shift+N and
// Ctrl+Shift+T are conventionally bound in a full browser) only when the
// host app provides that shell UI itself — Wails doesn't, so those combos
// are unclaimed here. Chosen over a Ctrl+Alt combo for the same AltGr
// composition reason as DefaultHotkeyAddClipboardCommand.
const DefaultHotkeyNewTerminal = "Ctrl+Shift+N"

// DefaultTerminalsSidebarWidthCh / DefaultCommandsSidebarWidthCh approximate
// the sidebars' original hardcoded pixel widths (10rem/160px and 220px) in
// `ch` units at the app's default UI font size — see AppConfig's field docs.
const DefaultTerminalsSidebarWidthCh = 20
const DefaultCommandsSidebarWidthCh = 30

// configDir returns the ~/.pairadmin directory path.
func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pairadmin")
}

// configPath returns the full path to ~/.pairadmin/config.yaml.
func configPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

// LoadAppConfig reads the application configuration from ~/.pairadmin/config.yaml.
// Returns an empty AppConfig (with no CustomPatterns) when the config file does not exist.
func LoadAppConfig() (*AppConfig, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir())
	v.SetDefault("custom_patterns", []CustomPattern{})
	v.SetDefault("remote_hosts", []RemoteHost{})
	v.SetDefault("pinned_commands", []PinnedCommand{})
	v.SetDefault("hotkey_add_clipboard_command", DefaultHotkeyAddClipboardCommand)
	v.SetDefault("hotkey_new_terminal", DefaultHotkeyNewTerminal)
	v.SetDefault("terminals_sidebar_width_ch", DefaultTerminalsSidebarWidthCh)
	v.SetDefault("commands_sidebar_width_ch", DefaultCommandsSidebarWidthCh)
	v.SetDefault("prompt_new_host_keys", false)
	// Missing config file is not an error — returns defaults.
	_ = v.ReadInConfig()
	var cfg AppConfig
	return &cfg, v.Unmarshal(&cfg)
}

// SaveAppConfig persists the application configuration to ~/.pairadmin/config.yaml.
// Creates the ~/.pairadmin/ directory if it does not exist (per Pitfall 6).
// Merges new fields without overwriting unrelated existing fields in the config file.
func SaveAppConfig(cfg *AppConfig) error {
	// Ensure directory exists before writing.
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configDir())
	_ = v.ReadInConfig() // Load existing values first — merge, don't overwrite

	v.Set("custom_patterns", cfg.CustomPatterns)
	v.Set("provider", cfg.Provider)
	v.Set("model", cfg.Model)
	v.Set("custom_prompt", cfg.CustomPrompt)
	v.Set("atspi_polling_ms", cfg.ATSPIPollingMs)
	v.Set("clipboard_clear_secs", cfg.ClipboardClearSecs)
	v.Set("hotkey_copy_last", cfg.HotkeyCopyLast)
	v.Set("hotkey_focus_window", cfg.HotkeyFocusWindow)
	v.Set("hotkey_add_clipboard_command", cfg.HotkeyAddClipboardCommand)
	v.Set("hotkey_new_terminal", cfg.HotkeyNewTerminal)
	v.Set("theme", cfg.Theme)
	v.Set("font_size", cfg.FontSize)
	v.Set("context_lines", cfg.ContextLines)
	v.Set("ollama_host", cfg.OllamaHost)
	v.Set("lmstudio_host", cfg.LMStudioHost)
	v.Set("remote_hosts", cfg.RemoteHosts)
	v.Set("terminals_sidebar_width_ch", cfg.TerminalsSidebarWidthCh)
	v.Set("commands_sidebar_width_ch", cfg.CommandsSidebarWidthCh)
	v.Set("pinned_commands", cfg.PinnedCommands)
	v.Set("prompt_new_host_keys", cfg.PromptNewHostKeys)
	return v.WriteConfigAs(configPath())
}
