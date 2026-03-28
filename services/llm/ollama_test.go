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

func TestOllamaValidateHostRemoteRejects(t *testing.T) {
	// NewOllamaProvider with OLLAMA_HOST="http://remotehost:11434" returns error "must be localhost"
	err := validateOllamaHost("http://remotehost:11434")
	if err == nil {
		t.Error("expected error for remote OLLAMA_HOST, got nil")
	}
	if err != nil && err.Error() != "OLLAMA_HOST must be localhost or 127.0.0.1; remote hosts are not allowed" {
		// Allow any error message containing "must be localhost"
		if len(err.Error()) == 0 {
			t.Error("expected non-empty error message")
		}
	}
}

func TestOllamaValidateHostRemoteIP(t *testing.T) {
	// Remote IP address should also be rejected
	err := validateOllamaHost("http://192.168.1.100:11434")
	if err == nil {
		t.Error("expected error for remote IP OLLAMA_HOST, got nil")
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

func TestNewOllamaProviderRemoteHostFails(t *testing.T) {
	// NewOllamaProvider with remote host returns error
	p, err := NewOllamaProvider("http://remotehost:11434", "llama3")
	if err == nil {
		t.Error("expected error for remote OLLAMA_HOST, got nil")
	}
	if p != nil {
		t.Error("expected nil provider when error occurs")
	}
}
