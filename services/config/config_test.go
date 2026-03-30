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

// TestAppConfig_FullRoundTrip verifies all Phase 5 fields survive a save/load cycle.
func TestAppConfig_FullRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	original := &AppConfig{
		Provider:          "openai",
		Model:             "gpt-4",
		CustomPrompt:      "Be helpful",
		ATSPIPollingMs:    1000,
		ClipboardClearSecs: 60,
		Theme:             "dark",
		FontSize:          14,
		ContextLines:      200,
		CustomPatterns:    []CustomPattern{{Name: "test", Regex: ".*", Action: "redact"}},
	}

	if err := SaveAppConfig(original); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}

	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error: %v", err)
	}

	if loaded.Provider != "openai" {
		t.Errorf("Provider: expected 'openai', got %q", loaded.Provider)
	}
	if loaded.Model != "gpt-4" {
		t.Errorf("Model: expected 'gpt-4', got %q", loaded.Model)
	}
	if loaded.CustomPrompt != "Be helpful" {
		t.Errorf("CustomPrompt: expected 'Be helpful', got %q", loaded.CustomPrompt)
	}
	if loaded.ATSPIPollingMs != 1000 {
		t.Errorf("ATSPIPollingMs: expected 1000, got %d", loaded.ATSPIPollingMs)
	}
	if loaded.ClipboardClearSecs != 60 {
		t.Errorf("ClipboardClearSecs: expected 60, got %d", loaded.ClipboardClearSecs)
	}
	if loaded.Theme != "dark" {
		t.Errorf("Theme: expected 'dark', got %q", loaded.Theme)
	}
	if loaded.FontSize != 14 {
		t.Errorf("FontSize: expected 14, got %d", loaded.FontSize)
	}
	if loaded.ContextLines != 200 {
		t.Errorf("ContextLines: expected 200, got %d", loaded.ContextLines)
	}
	if len(loaded.CustomPatterns) != 1 {
		t.Fatalf("CustomPatterns: expected 1, got %d", len(loaded.CustomPatterns))
	}
}

// TestSaveAppConfig_Merge verifies that SaveAppConfig merges without wiping existing fields.
func TestSaveAppConfig_Merge(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// First save: only Provider
	cfg1 := &AppConfig{Provider: "openai"}
	if err := SaveAppConfig(cfg1); err != nil {
		t.Fatalf("first SaveAppConfig() unexpected error: %v", err)
	}

	// Second save: add Model (load first, add field, then save)
	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error: %v", err)
	}
	loaded.Model = "gpt-4"
	if err := SaveAppConfig(loaded); err != nil {
		t.Fatalf("second SaveAppConfig() unexpected error: %v", err)
	}

	// Reload: both Provider and Model should be present
	reloaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("second LoadAppConfig() unexpected error: %v", err)
	}
	if reloaded.Provider != "openai" {
		t.Errorf("Provider: expected 'openai' after merge, got %q", reloaded.Provider)
	}
	if reloaded.Model != "gpt-4" {
		t.Errorf("Model: expected 'gpt-4' after merge, got %q", reloaded.Model)
	}
}

// TestSaveAppConfig_PreservesCustomPatternsOnNewFieldSave verifies that saving new fields keeps CustomPatterns.
func TestSaveAppConfig_PreservesCustomPatternsOnNewFieldSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Save with CustomPatterns and Provider
	cfg := &AppConfig{
		CustomPatterns: []CustomPattern{{Name: "test", Regex: ".*", Action: "redact"}},
		Provider:       "openai",
	}
	if err := SaveAppConfig(cfg); err != nil {
		t.Fatalf("SaveAppConfig() unexpected error: %v", err)
	}

	// Reload: both should be present
	loaded, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() unexpected error: %v", err)
	}
	if len(loaded.CustomPatterns) != 1 {
		t.Fatalf("CustomPatterns: expected 1, got %d", len(loaded.CustomPatterns))
	}
	if loaded.Provider != "openai" {
		t.Errorf("Provider: expected 'openai', got %q", loaded.Provider)
	}
}
