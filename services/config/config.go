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
	CustomPatterns []CustomPattern `mapstructure:"custom_patterns" yaml:"custom_patterns"`
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
func SaveAppConfig(cfg *AppConfig) error {
	// Ensure directory exists before writing.
	if err := os.MkdirAll(configDir(), 0o700); err != nil {
		return err
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.Set("custom_patterns", cfg.CustomPatterns)
	return v.WriteConfigAs(configPath())
}
