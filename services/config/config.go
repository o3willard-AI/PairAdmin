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
	Theme              string          `mapstructure:"theme" yaml:"theme"`
	FontSize           int             `mapstructure:"font_size" yaml:"font_size"`
	ContextLines       int             `mapstructure:"context_lines" yaml:"context_lines"`
	OllamaHost         string          `mapstructure:"ollama_host" yaml:"ollama_host"`
	LMStudioHost       string          `mapstructure:"lmstudio_host" yaml:"lmstudio_host"`
	RemoteHosts        []RemoteHost    `mapstructure:"remote_hosts" yaml:"remote_hosts"`
}

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
	v.Set("theme", cfg.Theme)
	v.Set("font_size", cfg.FontSize)
	v.Set("context_lines", cfg.ContextLines)
	v.Set("ollama_host", cfg.OllamaHost)
	v.Set("lmstudio_host", cfg.LMStudioHost)
	v.Set("remote_hosts", cfg.RemoteHosts)
	return v.WriteConfigAs(configPath())
}
