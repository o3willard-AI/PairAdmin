package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIProvider implements the Provider interface for OpenAI, OpenRouter, and LM Studio.
// OpenRouter: pass BaseURL="https://openrouter.ai/api/v1" and OPENROUTER_API_KEY.
// LM Studio: pass BaseURL="http://localhost:1234/v1" and empty apiKey.
type OpenAIProvider struct {
	apiKey  string
	baseURL string
	model   string
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
// An empty apiKey does not panic — it may be valid for local deployments (LM Studio).
// An empty baseURL uses the default OpenAI API endpoint.
func NewOpenAIProvider(apiKey, baseURL, model string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		model:   model,
	}
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// newClient creates an OpenAI client with the configured API key and optional base URL.
func (p *OpenAIProvider) newClient() openai.Client {
	opts := []option.RequestOption{
		option.WithAPIKey(p.apiKey),
	}
	if p.baseURL != "" {
		opts = append(opts, option.WithBaseURL(p.baseURL))
	}
	return openai.NewClient(opts...)
}

// Stream initiates a streaming chat completion request and returns a channel of chunks.
func (p *OpenAIProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 32)

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.model),
		Messages: convertMessagesToOpenAI(messages),
	}

	go func() {
		defer close(ch)

		client := p.newClient()
		stream := client.Chat.Completions.NewStreaming(ctx, params)
		defer stream.Close()

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					select {
					case ch <- StreamChunk{Text: delta.Content}:
					case <-ctx.Done():
						return
					}
				}
				if chunk.Choices[0].FinishReason == "stop" {
					ch <- StreamChunk{Done: true}
					return
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

		// Emit done if no explicit stop reason was encountered
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

// TestConnection verifies the API key and connectivity by listing models.
func (p *OpenAIProvider) TestConnection(ctx context.Context) error {
	client := p.newClient()
	_, err := client.Models.List(ctx)
	if err != nil {
		return fmt.Errorf("openai connection test failed: %w", err)
	}
	return nil
}

// convertMessagesToOpenAI converts the provider-agnostic Message slice to OpenAI SDK params.
func convertMessagesToOpenAI(messages []Message) []openai.ChatCompletionMessageParamUnion {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			result = append(result, openai.SystemMessage(m.Content))
		case RoleUser:
			result = append(result, openai.UserMessage(m.Content))
		case RoleAssistant:
			result = append(result, openai.AssistantMessage(m.Content))
		}
	}
	return result
}
