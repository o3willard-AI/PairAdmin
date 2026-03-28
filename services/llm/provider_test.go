package llm

import (
	"context"
	"testing"
)

// MockProvider is a test provider used to verify the Provider interface.
type MockProvider struct {
	name string
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Stream(ctx context.Context, messages []Message) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 2)
	go func() {
		defer close(ch)
		ch <- StreamChunk{Text: "hello", Done: false}
		ch <- StreamChunk{Done: true}
	}()
	return ch, nil
}

func (m *MockProvider) TestConnection(ctx context.Context) error {
	return nil
}

func TestProviderInterface(t *testing.T) {
	// Verify that MockProvider satisfies the Provider interface at compile time.
	var _ Provider = &MockProvider{name: "mock"}

	p := &MockProvider{name: "test-mock"}
	if p.Name() != "test-mock" {
		t.Errorf("expected name 'test-mock', got %q", p.Name())
	}

	ch, err := p.Stream(context.Background(), []Message{
		{Role: RoleUser, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []StreamChunk
	for c := range ch {
		chunks = append(chunks, c)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk from Stream")
	}

	if err := p.TestConnection(context.Background()); err != nil {
		t.Errorf("unexpected TestConnection error: %v", err)
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %q, want 'system'", RoleSystem)
	}
	if RoleUser != "user" {
		t.Errorf("RoleUser = %q, want 'user'", RoleUser)
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q, want 'assistant'", RoleAssistant)
	}
}

func TestMessageStruct(t *testing.T) {
	m := Message{Role: RoleUser, Content: "test content"}
	if m.Role != RoleUser {
		t.Errorf("expected RoleUser, got %q", m.Role)
	}
	if m.Content != "test content" {
		t.Errorf("expected 'test content', got %q", m.Content)
	}
}

func TestStreamChunkStruct(t *testing.T) {
	chunk := StreamChunk{Text: "hello", Done: false}
	if chunk.Text != "hello" {
		t.Errorf("expected 'hello', got %q", chunk.Text)
	}
	if chunk.Done {
		t.Error("expected Done=false")
	}
	doneChunk := StreamChunk{Done: true}
	if !doneChunk.Done {
		t.Error("expected Done=true")
	}
}
