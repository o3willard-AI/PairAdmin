// Package llm provides a provider-agnostic LLM streaming interface and adapters
// for OpenAI, Anthropic, and Ollama (with OpenRouter and LM Studio via OpenAI adapter).
package llm

import "context"

// Role represents the sender role in a conversation message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation.
type Message struct {
	Role    Role
	Content string
}

// StreamChunk is a single streaming token or a terminal signal.
type StreamChunk struct {
	Text  string
	Done  bool
	Error error
}

// Usage represents token consumption for a completed request.
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// Provider is the common interface all LLM adapters must satisfy.
type Provider interface {
	// Name returns a human-readable provider identifier (e.g. "openai", "anthropic", "ollama").
	Name() string

	// Stream initiates a streaming request and returns a channel of StreamChunks.
	// The channel is closed when streaming completes or an error occurs.
	// The final chunk may have Done=true or Error set.
	Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error)

	// TestConnection verifies the provider is reachable and credentials are valid.
	// Used by the Phase 5 settings dialog — implemented here to avoid an interface break.
	TestConnection(ctx context.Context) error
}
