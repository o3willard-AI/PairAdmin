package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- validateOllamaHost ---

func TestValidateOllamaHost_EmptyOK(t *testing.T) {
	if err := validateOllamaHost(""); err != nil {
		t.Errorf("expected empty host to be accepted (default localhost:11434), got: %v", err)
	}
}

func TestValidateOllamaHost_HTTPSchemesOK(t *testing.T) {
	for _, host := range []string{
		"http://localhost:11434",
		"https://ollama.example.com",
		"http://127.0.0.1:11434",
		"http://[::1]:11434",
		"http://192.168.1.100:11434", // remote hosts ARE allowed — legitimate use case
	} {
		if err := validateOllamaHost(host); err != nil {
			t.Errorf("validateOllamaHost(%q): expected accepted, got: %v", host, err)
		}
	}
}

func TestValidateOllamaHost_Rejects(t *testing.T) {
	cases := []struct {
		host string
		why  string
	}{
		{"ftp://x", "non-http(s) scheme"},
		{"ftp://192.168.1.100:11434", "non-http(s) scheme on remote"},
		{"://bad", "unparseable URL"},
		{"not a url at all %%", "unparseable URL with bad escape"},
	}
	for _, tc := range cases {
		if err := validateOllamaHost(tc.host); err == nil {
			t.Errorf("validateOllamaHost(%q): expected rejection (%s), got nil", tc.host, tc.why)
		}
	}
}

// --- NewOllamaProvider signature / construction (llm_service.go calls this) ---

func TestNewOllamaProvider_EmptyLocalhostRemoteSucceed(t *testing.T) {
	for _, host := range []string{"", "http://localhost:11434", "http://ollama.lan:11434"} {
		p, err := NewOllamaProvider(host, "llama3")
		if err != nil {
			t.Fatalf("NewOllamaProvider(%q): unexpected error: %v", host, err)
		}
		if p == nil {
			t.Fatalf("NewOllamaProvider(%q): expected non-nil provider", host)
		}
		if p.Name() != "ollama" {
			t.Errorf("expected Name() 'ollama', got %q", p.Name())
		}
	}
}

func TestNewOllamaProvider_RejectsInvalidScheme(t *testing.T) {
	if _, err := NewOllamaProvider("ftp://x", "llama3"); err == nil {
		t.Error("expected ftp:// host to be rejected")
	}
}

// --- Stream over NDJSON (httptest fake of POST /api/chat) ---

// ndjsonChatServer returns an httptest server that fakes Ollama's streaming
// /api/chat: one JSON object per line, each with message.content and a done
// flag on the final line. It records the request body for assertions.
func ndjsonChatServer(t *testing.T, lines []string, status int, captureBody *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if captureBody != nil {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			*captureBody = body
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(status)
		for _, line := range lines {
			_, _ = w.Write([]byte(line + "\n"))
		}
	}))
}

func TestOllamaStream_EmitsTextChunksInOrderAndExactlyOneDone(t *testing.T) {
	var captured map[string]any
	srv := ndjsonChatServer(t, []string{
		`{"model":"llama3","message":{"role":"assistant","content":"Hello"},"done":false}`,
		`{"model":"llama3","message":{"role":"assistant","content":", world"},"done":false}`,
		`{"model":"llama3","message":{"role":"assistant","content":""},"done":false}`,
		`{"model":"llama3","message":{"role":"assistant","content":"!"},"done":true}`,
		// Lines AFTER done must never be read or emitted — a broken parser
		// that ignores the done flag emits these too and fails the test.
		`{"model":"llama3","message":{"role":"assistant","content":"GHOST"},"done":false}`,
		`{"model":"llama3","message":{"role":"assistant","content":"GHOST2"},"done":true}`,
	}, http.StatusOK, &captured)
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := p.Stream(ctx, []Message{
		{Role: RoleSystem, Content: "be brief"},
		{Role: RoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var texts []string
	doneCount := 0
	var errChunk error
	for chunk := range ch {
		switch {
		case chunk.Error != nil:
			errChunk = chunk.Error
		case chunk.Done:
			doneCount++
		case chunk.Text != "":
			texts = append(texts, chunk.Text)
		default:
			// An empty Text chunk here is a contract violation: the spec says
			// do NOT emit empty Text chunks (they render as blank lines).
			t.Errorf("received empty Text chunk (must not be emitted)")
		}
	}

	if errChunk != nil {
		t.Fatalf("unexpected Error chunk: %v", errChunk)
	}
	joined := strings.Join(texts, "")
	if joined != "Hello, world!" {
		t.Errorf("expected text chunks 'Hello' ', world' '!' in order, got %q (%v)", joined, texts)
	}
	if doneCount != 1 {
		t.Errorf("expected exactly one Done chunk, got %d", doneCount)
	}

	// Request body shape: model, stream=true, mapped messages.
	if captured["model"] != "llama3" {
		t.Errorf("expected request model 'llama3', got %v", captured["model"])
	}
	if stream, ok := captured["stream"].(bool); !ok || !stream {
		t.Errorf("expected request stream=true, got %v", captured["stream"])
	}
	msgs, _ := captured["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 mapped messages, got %d", len(msgs))
	}
	first, _ := msgs[0].(map[string]any)
	if first["role"] != "system" || first["content"] != "be brief" {
		t.Errorf("expected first message role=system content='be brief', got %v", first)
	}
	second, _ := msgs[1].(map[string]any)
	if second["role"] != "user" || second["content"] != "hi" {
		t.Errorf("expected second message role=user content='hi', got %v", second)
	}
}

func TestOllamaStream_HTTP500SurfacesErrorChunk(t *testing.T) {
	srv := ndjsonChatServer(t, []string{`{"error":"model 'llama3' not found"}`}, http.StatusInternalServerError, nil)
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}

	ch, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream should return the channel before the HTTP round-trip, got error: %v", err)
	}

	var gotErr error
	sawDone := false
	for chunk := range ch {
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
		if chunk.Done {
			sawDone = true
		}
	}
	if gotErr == nil {
		t.Fatal("expected an Error chunk for HTTP 500, got none")
	}
	if !strings.Contains(gotErr.Error(), "500") {
		t.Errorf("expected the error to mention the HTTP status 500, got: %v", gotErr)
	}
	if sawDone {
		t.Error("expected no Done chunk after an error")
	}
}

func TestOllamaStream_ErrorLineSurfacesErrorChunk(t *testing.T) {
	// A 200 response whose NDJSON carries an "error" field must surface as an
	// error too (Ollama reports some failures in-stream).
	srv := ndjsonChatServer(t, []string{
		`{"error":"ollama engine died"}`,
	}, http.StatusOK, nil)
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}

	ch, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var gotErr error
	for chunk := range ch {
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
	}
	if gotErr == nil {
		t.Fatal("expected an Error chunk for an NDJSON error line, got none")
	}
	if !strings.Contains(gotErr.Error(), "ollama engine died") {
		t.Errorf("expected the in-stream error text, got: %v", gotErr)
	}
}

func TestOllamaStream_ContextCancelStopsWithoutDone(t *testing.T) {
	// A server that never finishes the stream: cancellation must end the read
	// loop without emitting Done.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher := w.(http.Flusher)
		_, _ = w.Write([]byte(`{"message":{"role":"assistant","content":"partial"},"done":false}` + "\n"))
		flusher.Flush()
		// Hold the stream open until the client goes away.
		<-r.Context().Done()
	}))
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := p.Stream(ctx, []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	sawText := false
	sawDone := false
	var gotErr error
	for chunk := range ch {
		if chunk.Text != "" {
			sawText = true
			cancel() // user navigates away mid-stream
		}
		if chunk.Done {
			sawDone = true
		}
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
	}
	if !sawText {
		t.Error("expected the partial text chunk before cancellation")
	}
	if sawDone {
		t.Error("expected no Done chunk after cancellation")
	}
	if gotErr != nil && !strings.Contains(gotErr.Error(), "context canceled") {
		t.Errorf("expected context-canceled (or silence), got: %v", gotErr)
	}
}

// --- TestConnection (GET /api/tags) ---

func TestOllamaTestConnection_200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "expected GET", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}
	if err := p.TestConnection(context.Background()); err != nil {
		t.Errorf("TestConnection against a healthy server: expected nil, got %v", err)
	}
}

func TestOllamaTestConnection_500Errors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p, err := NewOllamaProvider(srv.URL, "llama3")
	if err != nil {
		t.Fatalf("NewOllamaProvider: %v", err)
	}
	if err := p.TestConnection(context.Background()); err == nil {
		t.Error("TestConnection against a 500-ing server: expected an error, got nil")
	}
}
