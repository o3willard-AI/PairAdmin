// Package llm - internal tests for the Gemini provider (tests unexported
// buildRequest method and the SSE wire protocol against httptest fakes).
package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// withGeminiBaseURL points the provider at a fake endpoint for one test.
func withGeminiBaseURL(t *testing.T, url string) {
	t.Helper()
	orig := geminiBaseURL
	geminiBaseURL = url
	t.Cleanup(func() { geminiBaseURL = orig })
}

// capturedRequest records what the provider actually sent.
type capturedRequest struct {
	mu     sync.Mutex
	path   string
	apiKey string
	body   map[string]any
}

func (c *capturedRequest) record(r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.path = r.URL.Path
	c.apiKey = r.Header.Get("x-goog-api-key")
	_ = json.NewDecoder(r.Body).Decode(&c.body)
}

func (c *capturedRequest) snapshot() (path, apiKey string, body map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.path, c.apiKey, c.body
}

// geminiSSEServer fakes Gemini's streamGenerateContent SSE endpoint.
func geminiSSEServer(t *testing.T, status int, dataLines []string, holdStream bool, cap *capturedRequest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cap != nil {
			cap.record(r)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"code":429,"message":"resource exhausted"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		for _, line := range dataLines {
			_, _ = w.Write([]byte("data: " + line + "\n\n"))
			flusher.Flush()
		}
		if holdStream {
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// sseLine builds a GenerateContentResponse chunk with the given text and
// finishReason ("" = none).
func sseLine(text, finishReason string) string {
	m := map[string]any{
		"candidates": []map[string]any{{
			"content": map[string]any{
				"role":  "model",
				"parts": []map[string]any{{"text": text}},
			},
		}},
	}
	if finishReason != "" {
		m["candidates"].([]map[string]any)[0]["finishReason"] = finishReason
	}
	b, _ := json.Marshal(m)
	return string(b)
}

func TestGeminiProviderCreation(t *testing.T) {
	p := NewGeminiProvider("test-key", "gemini-2.0-flash")
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "gemini" {
		t.Errorf("expected name %q, got %q", "gemini", p.Name())
	}
}

func TestGeminiBuildRequest_MapsRoles(t *testing.T) {
	p := NewGeminiProvider("k", "gemini-2.0-flash")
	req := p.buildRequest([]Message{
		{Role: RoleSystem, Content: "You are a terminal assistant."},
		{Role: RoleUser, Content: "Hello"},
		{Role: RoleAssistant, Content: "Hi there"},
		{Role: RoleUser, Content: "Follow up"},
	})

	if req.SystemInstruction == nil || len(req.SystemInstruction.Parts) == 0 ||
		req.SystemInstruction.Parts[0].Text != "You are a terminal assistant." {
		t.Errorf("expected system message mapped to systemInstruction, got %+v", req.SystemInstruction)
	}
	if len(req.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(req.Contents))
	}
	wantRoles := []string{"user", "model", "user"}
	wantTexts := []string{"Hello", "Hi there", "Follow up"}
	for i, c := range req.Contents {
		if c.Role != wantRoles[i] {
			t.Errorf("content[%d]: expected role %q, got %q", i, wantRoles[i], c.Role)
		}
		if len(c.Parts) != 1 || c.Parts[0].Text != wantTexts[i] {
			t.Errorf("content[%d]: expected text %q, got %+v", i, wantTexts[i], c.Parts)
		}
	}
	// Mutation check: mapping RoleAssistant to anything other than
	// Gemini's "model" role (or leaving system in contents) fails this.
}

func TestGeminiStream_EmitsChunksInOrderAndExactlyOneDone(t *testing.T) {
	// The FINAL data line carries BOTH text and finishReason STOP — a
	// parser that checks finishReason before emitting text truncates the
	// response by one chunk (the same trap Ollama's done-line taught us).
	cap := &capturedRequest{}
	srv := geminiSSEServer(t, http.StatusOK, []string{
		sseLine("Hello", ""),
		sseLine(" world", ""),
		sseLine("!", "STOP"),
	}, false, cap)
	withGeminiBaseURL(t, srv.URL)

	p := NewGeminiProvider("test-key", "gemini-2.0-flash")
	ch, err := p.Stream(context.Background(), []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "hi"},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var texts []string
	doneCount := 0
	var streamErr error
	var seq []string // full chunk sequence: "t:<text>" or "done"
	for chunk := range ch {
		if chunk.Error != nil {
			streamErr = chunk.Error
			continue
		}
		if chunk.Done {
			doneCount++
			seq = append(seq, "done")
			continue
		}
		if chunk.Text != "" {
			texts = append(texts, chunk.Text)
			seq = append(seq, "t:"+chunk.Text)
		}
	}
	if streamErr != nil {
		t.Fatalf("unexpected stream error: %v", streamErr)
	}
	if len(texts) != 3 || texts[0] != "Hello" || texts[1] != " world" || texts[2] != "!" {
		t.Errorf("expected chunks [Hello, ' world', !], got %v", texts)
	}
	if doneCount != 1 {
		t.Errorf("expected exactly one Done, got %d", doneCount)
	}
	// Mutation check (load-bearing): Done must be the TERMINAL chunk and
	// the last data line's text must precede it — a parser that honors
	// finishReason before emitting the final token produces the sequence
	// [..., "done", "t:!"] and fails this exact-sequence assertion.
	wantSeq := []string{"t:Hello", "t: world", "t:!", "done"}
	if strings.Join(seq, "|") != strings.Join(wantSeq, "|") {
		t.Errorf("expected chunk sequence %v, got %v", wantSeq, seq)
	}

	// Wire-level assertions: endpoint, auth header, body mapping.
	path, key, body := cap.snapshot()
	if !strings.Contains(path, ":streamGenerateContent") {
		t.Errorf("expected :streamGenerateContent path, got %q", path)
	}
	if key != "test-key" {
		t.Errorf("expected x-goog-api-key header 'test-key', got %q", key)
	}
	if sys, ok := body["systemInstruction"].(map[string]any); !ok || sys == nil {
		t.Errorf("expected systemInstruction in request body, got %v", body)
	}
	if contents, ok := body["contents"].([]any); !ok || len(contents) != 1 {
		t.Errorf("expected 1 content (user only), got %v", body["contents"])
	}
}

func TestGeminiStream_HTTPError_SurfacesErrorChunkNotDone(t *testing.T) {
	srv := geminiSSEServer(t, http.StatusTooManyRequests, nil, false, nil)
	withGeminiBaseURL(t, srv.URL)

	p := NewGeminiProvider("test-key", "gemini-2.0-flash")
	ch, err := p.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var gotErr error
	doneCount := 0
	for chunk := range ch {
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
		if chunk.Done {
			doneCount++
		}
	}
	if gotErr == nil {
		t.Fatal("expected an error chunk for HTTP 429")
	}
	if !strings.Contains(gotErr.Error(), "429") || !strings.Contains(gotErr.Error(), "gemini") {
		t.Errorf("error should carry the provider name and HTTP status, got: %v", gotErr)
	}
	if strings.Contains(gotErr.Error(), "test-key") {
		t.Errorf("error must never leak the API key: %v", gotErr)
	}
	if doneCount != 0 {
		t.Errorf("a failed stream must not emit Done, got %d", doneCount)
	}
	// Mutation check: swallowing the non-200 status (treating any
	// response body as SSE) fails this — the body is a JSON error, not a
	// stream, and no error chunk surfaces.
}

func TestGeminiStream_ContextCancelStopsWithoutDone(t *testing.T) {
	srv := geminiSSEServer(t, http.StatusOK,
		[]string{sseLine("partial", "")}, true, nil)
	withGeminiBaseURL(t, srv.URL)

	p := NewGeminiProvider("test-key", "gemini-2.0-flash")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := p.Stream(ctx, []Message{{Role: RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	sawText := false
	doneCount := 0
	var gotErr error
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	for chunk := range ch {
		if chunk.Text != "" {
			sawText = true
		}
		if chunk.Done {
			doneCount++
		}
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
	}
	if !sawText {
		t.Error("expected the pre-cancel chunk to arrive")
	}
	if doneCount != 0 {
		t.Errorf("cancelled stream must not emit Done, got %d", doneCount)
	}
	if gotErr != nil {
		t.Errorf("cancelled stream ends silently (no error chunk), got %v", gotErr)
	}
}

func TestGeminiTestConnection_Success(t *testing.T) {
	cap := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cap.record(r)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"pong"}]}}]}`))
	}))
	defer srv.Close()
	withGeminiBaseURL(t, srv.URL)

	p := NewGeminiProvider("test-key", "gemini-2.0-flash")
	if err := p.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}

	path, key, body := cap.snapshot()
	if !strings.Contains(path, ":generateContent") {
		t.Errorf("expected :generateContent probe path, got %q", path)
	}
	if key != "test-key" {
		t.Errorf("expected x-goog-api-key header, got %q", key)
	}
	if len(body) == 0 {
		t.Error("expected a JSON probe body")
	}
}

func TestGeminiTestConnection_FailureCarriesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":401,"message":"API key not valid"}}`))
	}))
	defer srv.Close()
	withGeminiBaseURL(t, srv.URL)

	p := NewGeminiProvider("bad-key", "gemini-2.0-flash")
	err := p.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected an error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "gemini") || !strings.Contains(err.Error(), "401") {
		t.Errorf("error should carry provider name + status, got: %v", err)
	}
	if strings.Contains(err.Error(), "bad-key") {
		t.Errorf("error must never leak the API key: %v", err)
	}
	// Mutation check: removing the non-2xx check in TestConnection makes
	// the probe "succeed" against any reachable server, failing this.
}
