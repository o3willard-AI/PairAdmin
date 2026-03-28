package llm

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements the Provider interface for Anthropic Claude models.
// The system message is extracted to the top-level MessageNewParams.System field,
// not included in the messages array (which Anthropic's API requires).
type AnthropicProvider struct {
	apiKey string
	model  string
	client anthropic.Client
}

// NewAnthropicProvider creates a new Anthropic provider with the given API key and model.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: client,
	}
}

// Name returns the provider identifier.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// buildParams converts the provider-agnostic message slice into Anthropic MessageNewParams.
// System messages are extracted to params.System; other roles go into params.Messages.
func (p *AnthropicProvider) buildParams(messages []Message) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 2048,
	}

	var userMsgs []anthropic.MessageParam
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			params.System = []anthropic.TextBlockParam{{Text: m.Content}}
		case RoleUser:
			userMsgs = append(userMsgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case RoleAssistant:
			userMsgs = append(userMsgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	params.Messages = userMsgs
	return params
}

// Stream initiates a streaming message request and returns a channel of chunks.
func (p *AnthropicProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 32)

	params := p.buildParams(messages)

	go func() {
		defer close(ch)

		stream := p.client.Messages.NewStreaming(ctx, params)
		defer stream.Close()

		for stream.Next() {
			event := stream.Current()
			switch variant := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				delta := variant.Delta
				// Use the Type field to identify text deltas
				if delta.Type == "text_delta" {
					textDelta := delta.AsTextDelta()
					if textDelta.Text != "" {
						select {
						case ch <- StreamChunk{Text: textDelta.Text}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}

		if err := stream.Err(); err != nil {
			select {
			case ch <- StreamChunk{Error: err}:
			default:
			}
			return
		}

		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

// TestConnection verifies the API key and connectivity.
func (p *AnthropicProvider) TestConnection(ctx context.Context) error {
	// Make a minimal streaming request to verify credentials
	params := anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
		},
	}
	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()
	// Just try to get the first event to verify auth
	stream.Next()
	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic connection test failed: %w", err)
	}
	return nil
}
