// Package llm - internal test for anthropic provider (tests unexported buildParams method)
package llm

import (
	"testing"
)

func TestAnthropicProviderCreation(t *testing.T) {
	p := NewAnthropicProvider("test-key", "claude-3-5-sonnet-20241022")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got %q", p.Name())
	}
}

func TestAnthropicBuildParamsExtractsSystemMessage(t *testing.T) {
	// AnthropicProvider.buildParams extracts system message into params.System (not messages array)
	p := NewAnthropicProvider("test-key", "claude-3-5-sonnet-20241022")

	messages := []Message{
		{Role: RoleSystem, Content: "You are a helpful assistant."},
		{Role: RoleUser, Content: "Hello"},
	}

	params := p.buildParams(messages)

	// System should be in params.System
	if len(params.System) == 0 {
		t.Fatal("expected system message in params.System, got empty")
	}
	if params.System[0].Text != "You are a helpful assistant." {
		t.Errorf("expected system text 'You are a helpful assistant.', got %q", params.System[0].Text)
	}

	// System message should NOT be in params.Messages (only user/assistant roles allowed)
	// Verify we have fewer messages than total (1 system was extracted, 1 user remains)
	if len(params.Messages) != 1 {
		t.Errorf("expected 1 message in params.Messages (user only), got %d", len(params.Messages))
	}
}

func TestAnthropicBuildParamsNoSystemMessage(t *testing.T) {
	p := NewAnthropicProvider("test-key", "claude-3-5-sonnet-20241022")

	messages := []Message{
		{Role: RoleUser, Content: "Hello"},
	}

	params := p.buildParams(messages)

	if len(params.System) != 0 {
		t.Errorf("expected empty params.System when no system message, got %v", params.System)
	}
	if len(params.Messages) != 1 {
		t.Errorf("expected 1 user message, got %d", len(params.Messages))
	}
}

func TestAnthropicBuildParamsMultipleRoles(t *testing.T) {
	p := NewAnthropicProvider("test-key", "claude-3-5-sonnet-20241022")

	messages := []Message{
		{Role: RoleSystem, Content: "System prompt"},
		{Role: RoleUser, Content: "User question"},
		{Role: RoleAssistant, Content: "Assistant answer"},
		{Role: RoleUser, Content: "Follow up"},
	}

	params := p.buildParams(messages)

	// System extracted
	if len(params.System) == 0 || params.System[0].Text != "System prompt" {
		t.Error("system message not properly extracted")
	}

	// 3 messages in conversation (user, assistant, user)
	if len(params.Messages) != 3 {
		t.Errorf("expected 3 conversation messages, got %d", len(params.Messages))
	}
}
