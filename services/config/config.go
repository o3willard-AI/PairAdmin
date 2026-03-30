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
	return v.WriteConfigAs(configPath())
}
