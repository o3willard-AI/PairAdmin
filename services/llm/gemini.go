package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// geminiProvider implements the Provider interface for Google's Gemini API
// (generativelanguage.googleapis.com), speaking the generateContent wire
// protocol directly over net/http — no Google SDK, matching the Ollama
// adapter's self-owned-client approach (the HTTP surface is the contract).
// Auth is the x-goog-api-key header; the API key never appears in a URL,
// a log line, or an error message.
type geminiProvider struct {
	apiKey string
	model  string
	http   *http.Client
}

// geminiBaseURL is the API root. A package-level var (not a const) so tests
// can point the provider at an httptest server — the same injectable-seam
// convention as dialFunc / scanAllowed in services/scanner.
var geminiBaseURL = "https://generativelanguage.googleapis.com"

// NewGeminiProvider creates a new Gemini provider with the given API key
// and model. The (apiKey, model) order mirrors NewAnthropicProvider.
func NewGeminiProvider(apiKey, model string) *geminiProvider {
	return &geminiProvider{
		apiKey: apiKey,
		model:  model,
		http:   &http.Client{},
	}
}

// Name returns the provider identifier.
func (p *geminiProvider) Name() string {
	return "gemini"
}

// geminiPart / geminiContent / geminiRequest mirror Gemini's
// generateContent request body.
type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

// buildRequest converts the provider-agnostic message slice into Gemini's
// request shape: system messages go to the top-level systemInstruction
// field; user/assistant messages go to contents with Gemini's role names
// ("user" / "model" — not "assistant").
func (p *geminiProvider) buildRequest(messages []Message) geminiRequest {
	req := geminiRequest{Contents: []geminiContent{}}
	for _, m := range messages {
		switch m.Role {
		case RoleSystem:
			if req.SystemInstruction == nil {
				req.SystemInstruction = &geminiContent{Parts: []geminiPart{}}
			}
			req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, geminiPart{Text: m.Content})
		case RoleUser:
			req.Contents = append(req.Contents, geminiContent{Role: "user", Parts: []geminiPart{{Text: m.Content}}})
		case RoleAssistant:
			req.Contents = append(req.Contents, geminiContent{Role: "model", Parts: []geminiPart{{Text: m.Content}}})
		}
	}
	return req
}

// geminiResponse is the (streaming or not) GenerateContentResponse chunk.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// Stream initiates a streamGenerateContent request (SSE) and returns a
// channel of chunks. Channel semantics match the other adapters: Text
// chunks as tokens arrive, exactly one terminal Done on a clean finish, an
// Error chunk (never the key) on failure, and silence (no Done) on
// caller-side cancellation.
func (p *geminiProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 32)

	go func() {
		defer close(ch)
		// An unrecovered panic in any goroutine crashes the entire app, not
		// just this request — convert it into a chat error instead.
		defer func() {
			if r := recover(); r != nil {
				select {
				case ch <- StreamChunk{Error: fmt.Errorf("gemini stream panic: %v", r)}:
				default:
				}
			}
		}()

		if err := p.streamOnce(ctx, messages, ch); err != nil {
			select {
			case ch <- StreamChunk{Error: err}:
			default:
			}
		}
	}()

	return ch, nil
}

// streamOnce performs one SSE request and feeds chunks into ch. A nil
// return means the stream finished normally (Done already emitted or the
// caller cancelled); a non-nil return surfaces as an Error chunk.
func (p *geminiProvider) streamOnce(ctx context.Context, messages []Message, ch chan<- StreamChunk) error {
	body, err := json.Marshal(p.buildRequest(messages))
	if err != nil {
		return fmt.Errorf("gemini request encode: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse",
		strings.TrimRight(geminiBaseURL, "/"), p.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("gemini request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("gemini stream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, errorSnippet(resp.Body))
	}

	// SSE: one "data: {json}" line per GenerateContentResponse chunk.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	finished := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue // keep-alives, comments, blank separators
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var chunk geminiResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue // unknown event shapes must not kill the stream
		}
		for _, cand := range chunk.Candidates {
			// Emit text BEFORE honoring finishReason: Gemini's final data
			// line can carry both the last token and the STOP reason —
			// checking finishReason first truncates the response.
			for _, part := range cand.Content.Parts {
				if part.Text != "" {
					select {
					case ch <- StreamChunk{Text: part.Text}:
					case <-ctx.Done():
						return nil // caller cancelled: no Done, no error
					}
				}
			}
			if cand.FinishReason != "" {
				ch <- StreamChunk{Done: true}
				finished = true
			}
		}
		if finished {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		// Caller-side cancellation ends the stream silently — no Done, no
		// error chunk (the user navigated away; same convention as Ollama).
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("gemini stream read failed: %w", err)
	}

	// EOF without a finishReason (server died mid-response): still emit
	// Done so the chat pane finalizes — same convention as Ollama.
	ch <- StreamChunk{Done: true}
	return nil
}

// TestConnection verifies the API key and connectivity with a minimal
// generateContent probe. Any 2xx means reachable-and-authenticated; the
// probe body is never streamed to the UI.
func (p *geminiProvider) TestConnection(ctx context.Context) error {
	body, err := json.Marshal(p.buildRequest([]Message{{Role: RoleUser, Content: "ping"}}))
	if err != nil {
		return fmt.Errorf("gemini request encode: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent",
		strings.TrimRight(geminiBaseURL, "/"), p.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("gemini request build: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("gemini connection test failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gemini connection test failed (HTTP %d): %s", resp.StatusCode, errorSnippet(resp.Body))
	}
	return nil
}

// errorSnippet reads a bounded slice of an error response body for error
// context. It never echoes request headers, so the API key cannot leak.
func errorSnippet(r io.Reader) string {
	const max = 512
	buf := make([]byte, max)
	n, _ := r.Read(buf)
	snippet := strings.TrimSpace(string(buf[:n]))
	if len(snippet) >= max {
		snippet = snippet[:max]
	}
	if snippet == "" {
		return "(no body)"
	}
	return snippet
}
