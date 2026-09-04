package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ollamaProvider implements the Provider interface for Ollama deployments,
// speaking Ollama's stable HTTP API directly over net/http (POST /api/chat
// NDJSON streaming, GET /api/tags for reachability). No Ollama SDK: the HTTP
// surface is the contract, and owning the client keeps the dependency tree
// (and its server-side advisories) out of the app.
type ollamaProvider struct {
	host  string // base URL, always non-empty after NewOllamaProvider
	model string
	// token is the optional bearer token for authenticated remote Ollama
	// servers. Empty = no Authorization header (local instances don't need one).
	token string
	http  *http.Client
}

// defaultOllamaHost is Ollama's out-of-the-box listen address.
const defaultOllamaHost = "http://localhost:11434"

// validateOllamaHost validates the host URL format. An empty host is accepted
// (Ollama defaults to http://localhost:11434); non-empty hosts must parse as
// a URL with an http or https scheme. Remote hosts are allowed — pointing at
// an Ollama instance on another machine is a legitimate setup.
func validateOllamaHost(host string) error {
	if host == "" {
		return nil
	}
	u, err := url.Parse(host)
	if err != nil {
		return fmt.Errorf("invalid OLLAMA_HOST URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid OLLAMA_HOST scheme %q (want http or https)", u.Scheme)
	}
	return nil
}

// NewOllamaProvider creates a new Ollama provider for the given host and
// model. key is an optional bearer token for authenticated remote Ollama
// servers (empty = none). An empty host defaults to http://localhost:11434.
// The (key, host, model) order mirrors NewOpenAIProvider(key, baseURL, model)
// and NewAnthropicProvider(key, model).
func NewOllamaProvider(key, host, model string) (*ollamaProvider, error) {
	if err := validateOllamaHost(host); err != nil {
		return nil, err
	}
	if host == "" {
		host = defaultOllamaHost
	}
	return &ollamaProvider{
		host:  strings.TrimRight(host, "/"),
		model: model,
		token: key,
		http:  &http.Client{},
	}, nil
}

// Name returns the provider identifier.
func (p *ollamaProvider) Name() string {
	return "ollama"
}

// chatRequest / chatMessage mirror Ollama's /api/chat request body.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// chatResponse is one NDJSON line of Ollama's streaming /api/chat response:
// a message (with incremental content), a done flag on the final line, or an
// error field when something failed server-side.
type chatResponse struct {
	Message chatMessage `json:"message"`
	Done    bool        `json:"done"`
	Error   string      `json:"error,omitempty"`
}

// Stream initiates a streaming chat request and returns a channel of chunks.
// The HTTP read runs in a goroutine: POST {host}/api/chat, parse the NDJSON
// response line by line, emit one Text chunk per non-empty message.content
// and exactly one Done chunk on the final line. Any failure (non-2xx status,
// an in-stream "error" field, malformed lines, context cancellation, a panic)
// surfaces as an Error chunk instead of crashing the app.
func (p *ollamaProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 32)

	ollamaMsgs := make([]chatMessage, len(messages))
	for i, m := range messages {
		ollamaMsgs[i] = chatMessage{Role: string(m.Role), Content: m.Content}
	}
	body, err := json.Marshal(chatRequest{
		Model:    p.model,
		Messages: ollamaMsgs,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("encode ollama chat request: %w", err)
	}

	go func() {
		defer close(ch)
		// An unrecovered panic in any goroutine crashes the entire app, not
		// just this request — convert it into a chat error instead.
		defer func() {
			if r := recover(); r != nil {
				select {
				case ch <- StreamChunk{Error: fmt.Errorf("ollama stream panic: %v", r)}:
				default:
				}
			}
		}()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.host+"/api/chat", strings.NewReader(string(body)))
		if err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("build ollama chat request: %w", err)}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if p.token != "" {
			req.Header.Set("Authorization", "Bearer "+p.token)
		}

		resp, err := p.http.Do(req)
		if err != nil {
			// Context cancellation is the user navigating away — not an error
			// worth surfacing in the chat pane.
			if ctx.Err() != nil {
				return
			}
			ch <- StreamChunk{Error: fmt.Errorf("ollama chat request failed: %w", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			detail := strings.TrimSpace(string(respBody))
			if detail == "" {
				detail = resp.Status
			}
			ch <- StreamChunk{Error: fmt.Errorf("ollama chat failed: HTTP %d: %s", resp.StatusCode, detail)}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // NDJSON lines can exceed the 64KiB default
		for scanner.Scan() {
			if ctx.Err() != nil {
				return
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var cr chatResponse
			if err := json.Unmarshal([]byte(line), &cr); err != nil {
				ch <- StreamChunk{Error: fmt.Errorf("malformed ollama stream line: %w", err)}
				return
			}
			if cr.Error != "" {
				ch <- StreamChunk{Error: fmt.Errorf("ollama: %s", cr.Error)}
				return
			}
			// The FINAL line carries both the last token AND done=true — emit
			// its content first (dropping it truncates the response), then
			// terminate with Done. Empty content on a non-done line is
			// skipped; emitting it would render a blank chunk.
			if cr.Message.Content != "" {
				select {
				case ch <- StreamChunk{Text: cr.Message.Content}:
				case <-ctx.Done():
					return
				}
			}
			if cr.Done {
				ch <- StreamChunk{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			ch <- StreamChunk{Error: fmt.Errorf("read ollama stream: %w", err)}
			return
		}
		// Stream ended without a done line. Two very different causes:
		//   - the user cancelled (ctx done) → end silently, no Done — the
		//     consumer already knows the request was abandoned;
		//   - the server died mid-response → emit Done so the consumer's
		//     finalize path still runs; the alternative is a chat pane stuck
		//     on a blinking cursor forever.
		if ctx.Err() != nil {
			return
		}
		ch <- StreamChunk{Done: true}
	}()

	return ch, nil
}

// TestConnection verifies the Ollama server is reachable: GET {host}/api/tags
// (the models list). Any 2xx response counts as reachable.
func (p *ollamaProvider) TestConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.host+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("build ollama tags request: %w", err)
	}
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("ollama connection test failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("ollama connection test failed: HTTP %s", resp.Status)
	}
	return nil
}
