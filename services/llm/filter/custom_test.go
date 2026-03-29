package filter

import (
	"strings"
	"testing"
)

// TestNewCustomFilter_ValidRegex compiles without error for valid regex.
func TestNewCustomFilter_ValidRegex(t *testing.T) {
	_, err := NewCustomFilter([]CustomPatternInput{
		{Name: "test", Regex: `\bpassword\b`, Action: "redact"},
	})
	if err != nil {
		t.Errorf("NewCustomFilter() with valid regex returned error: %v", err)
	}
}

// TestNewCustomFilter_InvalidRegex returns descriptive error for invalid regex.
func TestNewCustomFilter_InvalidRegex(t *testing.T) {
	_, err := NewCustomFilter([]CustomPatternInput{
		{Name: "bad", Regex: `[invalid`, Action: "redact"},
	})
	if err == nil {
		t.Error("NewCustomFilter() with invalid regex: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("expected error to contain pattern name 'bad', got: %v", err)
	}
}

// TestCustomFilter_Apply_Redact replaces matches with [REDACTED:name].
func TestCustomFilter_Apply_Redact(t *testing.T) {
	f, err := NewCustomFilter([]CustomPatternInput{
		{Name: "apikey", Regex: `API_KEY=\S+`, Action: "redact"},
	})
	if err != nil {
		t.Fatalf("NewCustomFilter() error: %v", err)
	}

	content := "export API_KEY=supersecretvalue123\nsome other line"
	result, err := f.Apply(content)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if strings.Contains(result, "supersecretvalue123") {
		t.Error("Apply() with redact should remove the secret value")
	}
	if !strings.Contains(result, "[REDACTED:apikey]") {
		t.Errorf("Apply() with redact should contain '[REDACTED:apikey]', got: %q", result)
	}
}

// TestCustomFilter_Apply_Remove strips entire lines containing matches.
func TestCustomFilter_Apply_Remove(t *testing.T) {
	f, err := NewCustomFilter([]CustomPatternInput{
		{Name: "password", Regex: `password`, Action: "remove"},
	})
	if err != nil {
		t.Fatalf("NewCustomFilter() error: %v", err)
	}

	content := "line one\nset password=secret\nline three"
	result, err := f.Apply(content)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if strings.Contains(result, "password") {
		t.Error("Apply() with remove should strip lines containing 'password'")
	}
	if !strings.Contains(result, "line one") {
		t.Error("Apply() with remove should preserve non-matching lines")
	}
	if !strings.Contains(result, "line three") {
		t.Error("Apply() with remove should preserve non-matching lines")
	}
}

// TestCustomFilter_Apply_NoPatterns returns content unchanged when no patterns.
func TestCustomFilter_Apply_NoPatterns(t *testing.T) {
	f, err := NewCustomFilter([]CustomPatternInput{})
	if err != nil {
		t.Fatalf("NewCustomFilter() error: %v", err)
	}

	content := "some terminal content unchanged"
	result, err := f.Apply(content)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if result != content {
		t.Errorf("Apply() with no patterns: expected unchanged content, got %q", result)
	}
}

// TestCustomFilter_Apply_MultiplePatterns applies all patterns in order.
func TestCustomFilter_Apply_MultiplePatterns(t *testing.T) {
	f, err := NewCustomFilter([]CustomPatternInput{
		{Name: "key", Regex: `API_KEY=\S+`, Action: "redact"},
		{Name: "secret", Regex: `SECRET=\S+`, Action: "redact"},
	})
	if err != nil {
		t.Fatalf("NewCustomFilter() error: %v", err)
	}

	content := "API_KEY=abc123 SECRET=xyz789 safe_content"
	result, err := f.Apply(content)
	if err != nil {
		t.Fatalf("Apply() error: %v", err)
	}
	if strings.Contains(result, "abc123") {
		t.Error("Apply() should redact API key value")
	}
	if strings.Contains(result, "xyz789") {
		t.Error("Apply() should redact secret value")
	}
	if !strings.Contains(result, "[REDACTED:key]") {
		t.Error("Apply() should contain '[REDACTED:key]'")
	}
	if !strings.Contains(result, "[REDACTED:secret]") {
		t.Error("Apply() should contain '[REDACTED:secret]'")
	}
	if !strings.Contains(result, "safe_content") {
		t.Error("Apply() should preserve non-matching content")
	}
}
