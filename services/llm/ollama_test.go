package llm

import (
	"testing"
)

func TestOllamaValidateHostEmpty(t *testing.T) {
	// NewOllamaProvider with OLLAMA_HOST="" succeeds (default localhost)
	err := validateOllamaHost("")
	if err != nil {
		t.Errorf("expected nil error for empty OLLAMA_HOST, got: %v", err)
	}
}

func TestOllamaValidateHostLocalhost(t *testing.T) {
	// NewOllamaProvider with OLLAMA_HOST="http://localhost:11434" succeeds
	err := validateOllamaHost("http://localhost:11434")
	if err != nil {
		t.Errorf("expected nil error for localhost host, got: %v", err)
	}
}

func TestOllamaValidateHostLoopback(t *testing.T) {
	// NewOllamaProvider with OLLAMA_HOST="http://127.0.0.1:11434" succeeds
	err := validateOllamaHost("http://127.0.0.1:11434")
	if err != nil {
		t.Errorf("expected nil error for 127.0.0.1 host, got: %v", err)
	}
}

func TestOllamaValidateHostIPv6Loopback(t *testing.T) {
	// NewOllamaProvider with OLLAMA_HOST="http://[::1]:11434" succeeds
	err := validateOllamaHost("http://[::1]:11434")
	if err != nil {
		t.Errorf("expected nil error for ::1 host, got: %v", err)
	}
}

func TestOllamaValidateHostRemoteAccepts(t *testing.T) {
	// Remote hosts are now allowed (user-configured in settings UI)
	err := validateOllamaHost("http://remotehost:11434")
	if err != nil {
		t.Errorf("expected nil error for remote host, got: %v", err)
	}
}

func TestOllamaValidateHostRemoteIPAccepts(t *testing.T) {
	// Remote IP addresses are now allowed
	err := validateOllamaHost("http://192.168.1.100:11434")
	if err != nil {
		t.Errorf("expected nil error for remote IP, got: %v", err)
	}
}

func TestNewOllamaProviderEmptyHost(t *testing.T) {
	// NewOllamaProvider with empty host succeeds
	p, err := NewOllamaProvider("", "llama3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got %q", p.Name())
	}
}

func TestNewOllamaProviderLocalhostHost(t *testing.T) {
	// NewOllamaProvider with localhost host succeeds
	p, err := NewOllamaProvider("http://127.0.0.1:11434", "llama3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}

func TestNewOllamaProviderRemoteHostSucceeds(t *testing.T) {
	// NewOllamaProvider with remote host now succeeds (user-configured)
	p, err := NewOllamaProvider("http://remotehost:11434", "llama3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
}
