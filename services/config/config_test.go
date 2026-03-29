package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadAppConfig_EmptyWhenNoFile returns empty CustomPatterns when no config file exists.
func TestLoadAppConfig_EmptyWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadAppConfig() returned nil config")
	}
	if len(cfg.CustomPatterns) != 0 {
		t.Errorf("expected 0 custom patterns, got %d", len(cfg.CustomPatterns))
	}
}

// TestSaveAndLoadAppConfig_RoundTrip saves custom_patterns to YAML file and reads them back.
func TestSaveAndLoadAppConfig_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &AppConfig{
		CustomPatterns: []CustomPattern{
			{Name: "test-key", Regex: `TEST_KEY=\S+`, Action: "redact"},
			{Name: "password", Regex: `password:\s*\S+`, Action: "remove"},
		},
	}

	if err := SaveAppConfig(original); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}

	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() after save unexpected error: %v", err)
	}

	if len(loaded.CustomPatterns) != 2 {
		t.Fatalf("expected 2 custom patterns, got %d", len(loaded.CustomPatterns))
	}
	if loaded.CustomPatterns[0].Name != "test-key" {
		t.Errorf("expected pattern name 'test-key', got %q", loaded.CustomPatterns[0].Name)
	}
	if loaded.CustomPatterns[0].Regex != `TEST_KEY=\S+` {
		t.Errorf("expected regex 'TEST_KEY=\\S+', got %q", loaded.CustomPatterns[0].Regex)
	}
	if loaded.CustomPatterns[0].Action != "redact" {
		t.Errorf("expected action 'redact', got %q", loaded.CustomPatterns[0].Action)
	}
	if loaded.CustomPatterns[1].Name != "password" {
		t.Errorf("expected pattern name 'password', got %q", loaded.CustomPatterns[1].Name)
	}
	if loaded.CustomPatterns[1].Action != "remove" {
		t.Errorf("expected action 'remove', got %q", loaded.CustomPatterns[1].Action)
	}
}

// TestSaveAppConfig_CreatesDirectory verifies that SaveAppConfig creates ~/.pairadmin/ if it doesn't exist.
func TestSaveAppConfig_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &AppConfig{}
	if err := SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}

	expectedDir := filepath.Join(tmpDir, ".pairadmin")
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", expectedDir, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory", expectedDir)
	}

	expectedFile := filepath.Join(expectedDir, "config.yaml")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("expected config file %s to exist: %v", expectedFile, err)
	}
}

// TestLoadAppConfig_RoundTripPreservesFields ensures name, regex, action fields are all preserved.
func TestLoadAppConfig_RoundTripPreservesFields(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &AppConfig{
		CustomPatterns: []CustomPattern{
			{Name: "my-secret", Regex: `SECRET_[A-Z]+`, Action: "redact"},
		},
	}

	if err := SaveAppConfig(original); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}

	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error: %v", err)
	}

	if len(loaded.CustomPatterns) != 1 {
		t.Fatalf("expected 1 pattern, got %d", len(loaded.CustomPatterns))
	}
	p := loaded.CustomPatterns[0]
	if p.Name != "my-secret" {
		t.Errorf("Name: expected 'my-secret', got %q", p.Name)
	}
	if p.Regex != `SECRET_[A-Z]+` {
		t.Errorf("Regex: expected 'SECRET_[A-Z]+', got %q", p.Regex)
	}
	if p.Action != "redact" {
		t.Errorf("Action: expected 'redact', got %q", p.Action)
	}
}
