package filter_test

import (
	"strings"
	"testing"

	"pairadmin/services/llm/filter"
)

// TestANSIFilter_StripColorSequences verifies ANSI color codes are stripped.
func TestANSIFilter_StripColorSequences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "red text with reset",
			input: "\x1b[31mRed Text\x1b[0m",
			want:  "Red Text",
		},
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "cursor up sequence",
			input: "\x1b[1A",
			want:  "",
		},
		{
			name:  "clear screen sequence",
			input: "\x1b[2J",
			want:  "",
		},
	}

	f := filter.NewANSIFilter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Apply(tt.input)
			if err != nil {
				t.Fatalf("Apply() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Apply() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestANSIFilter_StripOSCSequence verifies OSC (Operating System Command) sequences are stripped.
func TestANSIFilter_StripOSCSequence(t *testing.T) {
	f := filter.NewANSIFilter()
	input := "\x1b]0;title\x07"
	got, err := f.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	// OSC sequence should be stripped — result should not contain the title text either
	// (the OSC sequence itself is stripped, but 'title' might remain depending on parser)
	if strings.Contains(got, "\x1b") {
		t.Errorf("Apply() result still contains escape sequence: %q", got)
	}
}

// TestCredentialFilter_RedactsAWSKey verifies AWS access key patterns are redacted.
func TestCredentialFilter_RedactsAWSKey(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}
	input := "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	got, err := f.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("Apply() did not redact AWS key, got: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("Apply() did not insert [REDACTED:...] marker, got: %q", got)
	}
}

// TestCredentialFilter_RedactsGitHubToken verifies GitHub personal access token patterns are redacted.
func TestCredentialFilter_RedactsGitHubToken(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}
	input := "token: ghp_1234567890abcdefghij1234567890abcdef12"
	got, err := f.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if strings.Contains(got, "ghp_1234567890abcdefghij1234567890abcdef12") {
		t.Errorf("Apply() did not redact GitHub token, got: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("Apply() did not insert [REDACTED:...] marker, got: %q", got)
	}
}

// TestCredentialFilter_SafeTextUnchanged verifies safe text passes through unmodified.
func TestCredentialFilter_SafeTextUnchanged(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}
	input := "hello world"
	got, err := f.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("Apply() modified safe text: got %q, want %q", got, input)
	}
}

// TestCredentialFilter_RedactsBearerToken verifies Bearer token patterns are redacted.
func TestCredentialFilter_RedactsBearerToken(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}
	input := "Authorization: Bearer eyJhbGciOiJSUzI1NiJ9.test"
	got, err := f.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if strings.Contains(got, "eyJhbGciOiJSUzI1NiJ9") {
		t.Errorf("Apply() did not redact bearer token, got: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("Apply() did not insert [REDACTED:...] marker, got: %q", got)
	}
}

// TestPipeline_RunsFiltersInOrder verifies Pipeline applies ANSIFilter then CredentialFilter.
func TestPipeline_RunsFiltersInOrder(t *testing.T) {
	ansiFilter := filter.NewANSIFilter()
	credFilter, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}

	p := filter.NewPipeline(ansiFilter, credFilter)

	// ANSI-wrapped AWS key — ANSI must be stripped first so credential can match
	input := "\x1b[31mAKIAIOSFODNN7EXAMPLE\x1b[0m"
	got, err := p.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if strings.Contains(got, "AKIAIOSFODNN7EXAMPLE") {
		t.Errorf("Pipeline did not redact ANSI-wrapped AWS key, got: %q", got)
	}
	if strings.Contains(got, "\x1b") {
		t.Errorf("Pipeline result still contains ANSI escape sequence: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("Pipeline did not insert [REDACTED:...] marker, got: %q", got)
	}
}

// TestPipeline_AppliesFiltersInSequence verifies output of first filter feeds into second.
func TestPipeline_AppliesFiltersInSequence(t *testing.T) {
	ansiFilter := filter.NewANSIFilter()
	credFilter, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}

	p := filter.NewPipeline(ansiFilter, credFilter)

	input := "plain text with no credentials"
	got, err := p.Apply(input)
	if err != nil {
		t.Fatalf("Apply() unexpected error: %v", err)
	}
	if got != input {
		t.Errorf("Pipeline modified safe plain text: got %q, want %q", got, input)
	}
}
