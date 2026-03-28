package llm

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	ollamaapi "github.com/ollama/ollama/api"
)

// OllamaProvider implements the Provider interface for local Ollama deployments.
// OLLAMA_HOST must be localhost, 127.0.0.1, or ::1 — remote hosts are rejected
// to prevent accidental transmission of terminal data over the network.
type OllamaProvider struct {
	client *ollamaapi.Client
	model  string
}

// validateOllamaHost returns an error if the host URL is not a localhost address.
// An empty host is accepted (Ollama defaults to localhost:11434).
func validateOllamaHost(host string) error {
	if host == "" {
		return nil
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("invalid OLLAMA_HOST URL: %w", err)
	}
	hostname := parsed.Hostname()
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
		return nil
	default:
		return fmt.Errorf("OLLAMA_HOST must be localhost or 127.0.0.1; remote hosts are not allowed")
	}
}

// NewOllamaProvider creates a new Ollama provider.
// Returns an error if host is non-empty and not a localhost address.
func NewOllamaProvider(host, model string) (*OllamaProvider, error) {
	if err := validateOllamaHost(host); err != nil {
		return nil, err
	}

	var clientURL *url.URL
	if host != "" {
		var err error
		clientURL, err = url.Parse(host)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OLLAMA_HOST: %w", err)
		}
	} else {
		// Default Ollama URL
		clientURL = &url.URL{
			Scheme: "http",
			Host:   "localhost:11434",
		}
	}

	client := ollamaapi.NewClient(clientURL, nil)
	return &OllamaProvider{
		client: client,
		model:  model,
	}, nil
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Stream initiates a streaming chat request and returns a channel of chunks.
// The Ollama SDK uses a callback-based API; this method wraps it into a channel.
func (p *OllamaProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 32)

	req := buildOllamaRequest(p.model, messages)

	go func() {
		defer close(ch)

		err := p.client.Chat(ctx, req, func(resp ollamaapi.ChatResponse) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if resp.Done {
				ch <- StreamChunk{Done: true}
				return nil
			}
			select {
			case ch <- StreamChunk{Text: resp.Message.Content}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case ch <- StreamChunk{Error: err}:
			default:
			}
		}
	}()

	return ch, nil
}

// TestConnection verifies the Ollama server is reachable by listing local models.
func (p *OllamaProvider) TestConnection(ctx context.Context) error {
	_, err := p.client.List(ctx)
	if err != nil {
		return fmt.Errorf("ollama connection test failed: %w", err)
	}
	return nil
}

// buildOllamaRequest converts provider-agnostic messages to an Ollama ChatRequest.
func buildOllamaRequest(model string, messages []Message) *ollamaapi.ChatRequest {
	var ollamaMsgs []ollamaapi.Message
	for _, m := range messages {
		ollamaMsgs = append(ollamaMsgs, ollamaapi.Message{
			Role:    string(m.Role),
			Content: m.Content,
		})
	}
	stream := true
	return &ollamaapi.ChatRequest{
		Model:    model,
		Messages: ollamaMsgs,
		Stream:   &stream,
	}
}
