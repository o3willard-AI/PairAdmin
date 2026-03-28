package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProviderCreation(t *testing.T) {
	// NewOpenAIProvider with empty API key does not panic; Stream returns channel
	p := NewOpenAIProvider("", "", "gpt-4")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", p.Name())
	}
}

func TestOpenAIProviderCustomBaseURL(t *testing.T) {
	// NewOpenAIProvider with custom BaseURL sets the base URL (covers OpenRouter + LM Studio)
	customURL := "https://openrouter.ai/api/v1"
	p := NewOpenAIProvider("test-key", customURL, "gpt-4")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.baseURL != customURL {
		t.Errorf("expected baseURL %q, got %q", customURL, p.baseURL)
	}
}

func TestOpenAIProviderStreamReturnsChan(t *testing.T) {
	// Create a mock OpenAI streaming server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Write a minimal SSE stream
		chunk := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion.chunk",
			"model":   "gpt-4",
			"choices": []map[string]interface{}{{"index": 0, "delta": map[string]string{"content": "Hello"}, "finish_reason": nil}},
		}
		data, _ := json.Marshal(chunk)
		w.Write([]byte("data: " + string(data) + "\n\n")) //nolint:errcheck
		flusher.Flush()

		doneChunk := map[string]interface{}{
			"id":      "chatcmpl-123",
			"object":  "chat.completion.chunk",
			"model":   "gpt-4",
			"choices": []map[string]interface{}{{"index": 0, "delta": map[string]string{}, "finish_reason": "stop"}},
		}
		doneData, _ := json.Marshal(doneChunk)
		w.Write([]byte("data: " + string(doneData) + "\n\n")) //nolint:errcheck
		flusher.Flush()

		w.Write([]byte("data: [DONE]\n\n")) //nolint:errcheck
		flusher.Flush()
	}))
	defer server.Close()

	p := NewOpenAIProvider("test-key", server.URL, "gpt-4")

	ctx := context.Background()
	ch, err := p.Stream(ctx, []Message{{Role: RoleUser, Content: "say hello"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Drain the channel
	for range ch {
	}
}

func TestOpenAIProviderWithLMStudioURL(t *testing.T) {
	// LM Studio uses OpenAI adapter with local URL and empty key
	lmStudioURL := "http://localhost:1234/v1"
	p := NewOpenAIProvider("", lmStudioURL, "local-model")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.baseURL != lmStudioURL {
		t.Errorf("expected baseURL %q, got %q", lmStudioURL, p.baseURL)
	}
}
