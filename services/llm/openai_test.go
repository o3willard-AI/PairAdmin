package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// --- TestConnection (POST /chat/completions, NOT the /models catalog) ---
//
// PA-01: TestConnection used to call client.Models.List. That probe cannot
// fail for a family of providers whose catalog endpoints are unauthenticated
// (OpenRouter returns 200 OK for a garbage or revoked key), so it reported
// "Connected" for dead credentials. These tests pin the inference-path probe.

// A minimal non-streaming chat completion body, which is all the SDK needs to
// consider the response successful.
const minimalCompletionJSON = `{
  "id": "chatcmpl-test",
  "object": "chat.completion",
  "created": 1700000000,
  "model": "test-model",
  "choices": [
    {"index": 0, "message": {"role": "assistant", "content": "hi"}, "finish_reason": "stop"}
  ]
}`

// Positive control: a valid completion makes TestConnection succeed.
//
// Mutation check: replacing the chat-completion call with the old
// `client.Models.List` makes this test fail — the path/method/auth assertions
// below see a GET to /models instead of a credentialed POST to
// /chat/completions. The NEGATIVE control is still the load-bearing one: it is
// the only assertion that fails on a revert for the *right* reason (a dead key
// reported as connected), so read its comment before trusting a green run here.
func TestOpenAIProviderTestConnection_ValidCompletion(t *testing.T) {
	var gotPath, gotMethod, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(minimalCompletionJSON)) //nolint:errcheck
	}))
	defer server.Close()

	p := NewOpenAIProvider("good-key", server.URL, "test-model")
	if err := p.TestConnection(context.Background()); err != nil {
		t.Fatalf("expected nil for a valid completion, got %v", err)
	}

	// Assert the probe actually went to the inference endpoint with credentials
	// attached — otherwise this test could pass without testing anything.
	if gotPath != "/chat/completions" {
		t.Errorf("expected the probe to hit /chat/completions, got %q", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotAuth == "" {
		t.Error("expected an Authorization header on the probe request")
	}
}

// Negative control — THE ACTUAL BUG (PA-01).
//
// This server reproduces OpenRouter's literal behavior: the catalog endpoint
// succeeds for ANY credential, including a dead one, while the inference
// endpoint is gated and returns 401. A catalog-based probe sees 200 and reports
// a healthy connection for a revoked key.
//
// Mutation check: reverting TestConnection to `client.Models.List(ctx)` makes
// this test FAIL — the probe would hit /models, get 200, and return nil. This
// is the single assertion that distinguishes the fix from the bug, so it must
// fail on any revert to the catalog probe.
func TestOpenAIProviderTestConnection_CatalogOpenButInferenceUnauthenticated(t *testing.T) {
	var probed []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probed = append(probed, r.URL.Path)
		switch r.URL.Path {
		case "/models":
			// Unauthenticated catalog: 200 OK for a garbage key, exactly as
			// OpenRouter behaves.
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"object":"list","data":[]}`)) //nolint:errcheck
		case "/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":{"message":"Invalid API key","type":"auth_error"}}`)) //nolint:errcheck
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	p := NewOpenAIProvider("revoked-key", server.URL, "test-model")

	err := p.TestConnection(context.Background())
	if err == nil {
		t.Fatal("TestConnection returned nil for a revoked key: the /models catalog " +
			"returned 200 so the probe cannot detect a dead credential (the PA-01 bug)")
	}
	if !strings.Contains(err.Error(), "openai connection test failed") {
		t.Errorf("expected the error to be wrapped as an openai connection failure, got %v", err)
	}

	// The catalog must not even have been consulted: if it was, the probe is
	// still partly trusting the endpoint that cannot fail.
	for _, path := range probed {
		if path == "/models" {
			t.Error("TestConnection still probes the /models catalog; that endpoint is " +
				"unauthenticated for several providers and cannot detect a bad key")
		}
	}
}
