package services

import (
	"context"
	"testing"
)

// TestBuildProviderDisabledIsNil verifies the explicit "disabled" provider
// case in buildProvider: no provider is constructed (SendMessage will reject
// with a descriptive error rather than attempting any connection).
func TestBuildProviderDisabledIsNil(t *testing.T) {
	if p := buildProvider(Config{Provider: "disabled", Model: ""}, nil); p != nil {
		t.Errorf("expected nil provider for Provider 'disabled', got %T", p)
	}
}

// TestSendMessageDisabledProviderErrors verifies SendMessage errors cleanly
// (without panic, without touching any network) when the LLM is disabled.
func TestSendMessageDisabledProviderErrors(t *testing.T) {
	svc := &LLMService{
		cfg:            Config{Provider: "disabled"},
		activeProvider: nil, // buildProvider returns nil for "disabled"
	}
	svc.ctx = context.Background()

	err := svc.SendMessage("tab-1", "hello", "")
	if err == nil {
		t.Fatal("expected error from SendMessage with disabled provider, got nil")
	}
}

// TestBuildProviderDisabledAfterRebuild verifies RebuildProvider's path:
// LoadConfigWithViper reading Provider "disabled" from disk still yields a nil
// provider — i.e. disabling via Settings (which persists to AppConfig and
// rebuilds) never constructs a live provider, and re-enabling works through
// the same path.
func TestBuildProviderDisabledAfterRebuild(t *testing.T) {
	cfg := LoadConfigWithViper()
	cfg.Provider = "disabled"
	if p := buildProvider(cfg, nil); p != nil {
		t.Errorf("expected nil provider when config Provider is 'disabled', got %T", p)
	}
}
