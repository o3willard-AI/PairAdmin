package filter_test

import (
	"os"
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

// TestCredentialFilter_RedactsExpandedPatterns exercises the expanded (R-06)
// credential pattern set. Each case drives a REAL secret through Apply and
// asserts (a) the secret is gone and (b) a [REDACTED:...] marker is present.
func TestCredentialFilter_RedactsExpandedPatterns(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}

	tests := []struct {
		name   string
		input  string
		secret string // must be absent from the output
	}{
		{
			name:   "gitlab personal access token",
			input:  "token: glpat-clintru5svolve9akhzyf40madeup",
			secret: "glpat-clintru5svolve9akhzyf40madeup",
		},
		{
			name:   "slack bot token",
			input:  "SLACK_BOT_TOKEN=xoxb-1234567890-ABCDEFGHIJKL",
			secret: "xoxb-1234567890-ABCDEFGHIJKL",
		},
		{
			name:   "slack app-level token",
			input:  "token=xoxp-123123123-123123123-abcdefghijkl",
			secret: "xoxp-123123123-123123123-abcdefghijkl",
		},
		{
			name:   "google api key",
			input:  "key=AIzaSyA1234567890ABcdefghijklmnopqrstuv",
			secret: "AIzaSyA1234567890ABcdefghijklmnopqrstuv",
		},
		{
			name: "google service account private key",
			input: `{
  "type": "service_account",
  "project_id": "my-proj",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFA\n-----END PRIVATE KEY-----\n"
}`,
			secret: "MIIEvQIBADANBgkqhkiG9w0BAQEFA",
		},
		{
			name:   "azure storage account key",
			input:  "DefaultEndpointsProtocol=https;AccountName=store;AccountKey=abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/==;EndpointSuffix=core.windows.net",
			secret: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789+/==",
		},
		{
			name:   "azure shared access key",
			input:  "ServiceBusConnectionString=Endpoint=sb://x.servicebus.windows.net/;SharedAccessKeyName=root;SharedAccessKey=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd",
			secret: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd",
		},
		{
			name:   "bare jwt (no bearer prefix)",
			input:  "session cookie: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			secret: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
		},
		{
			name: "rsa private key block",
			input: `-----BEGIN RSA PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCy915irt
-----END RSA PRIVATE KEY-----`,
			secret: "MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCy915irt",
		},
		{
			name:   "password assignment",
			input:  "export DB_PASSWORD=supersecretvalue123",
			secret: "supersecretvalue123",
		},
		{
			name:   "passwd assignment",
			input:  "passwd=letmein-now-please",
			secret: "letmein-now-please",
		},
		{
			name:   "connection string with embedded credentials",
			input:  "mongodb://admin:hunter2secret@db.example.com:27017/mydb",
			secret: "admin:hunter2secret@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Apply(tt.input)
			if err != nil {
				t.Fatalf("Apply() unexpected error: %v", err)
			}
			if strings.Contains(got, tt.secret) {
				t.Errorf("Apply() did not redact secret; output contains %q: %q", tt.secret, got)
			}
			if !strings.Contains(got, "[REDACTED:") {
				t.Errorf("Apply() did not insert a [REDACTED:...] marker: %q", got)
			}
		})
	}
}

// TestCredentialFilter_ExpandedNoFalsePositives drives SAFE text through
// Apply and asserts it passes through byte-for-byte unchanged. A false
// positive that strips legitimate terminal content is worse than a miss, so
// every new pattern needs a negative case here.
func TestCredentialFilter_ExpandedNoFalsePositives(t *testing.T) {
	f, err := filter.NewCredentialFilter()
	if err != nil {
		t.Fatalf("NewCredentialFilter() error: %v", err)
	}

	safe := []struct {
		name  string
		input string
	}{
		{name: "gitlab-name-not-token", input: "glpat is not a token without a 20+ char suffix"},
		{name: "gitlab-token-too-short", input: "glpat-xyz"},
		{name: "slack-xoxo-not-token", input: "xoxo-1234-5678 (an emoji hangover)"},
		{name: "slack-xoxb-too-short", input: "xoxb-abc"},
		{name: "google-key-too-short", input: "prefix AIza12 is not a key"},
		{name: "service-account-without-key", input: `{"type": "service_account", "project_id": "p"}`},
		{name: "azure-accountkey-short", input: "AccountKey=short is not a storage key"},
		{name: "azure-sas-short", input: "SharedAccessKey=abc123"},
		{name: "jwt-single-segment", input: "eyJhbGciOiJIUzI1NiJ9 is not a full jwt"},
		{name: "jwt-two-segments", input: "eyJh.eyJzdWIiLCJqdGki only two segments"},
		{name: "public-key-block", input: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEF\n-----END PUBLIC KEY-----"},
		{name: "password-short", input: "password=x is too short to redact"},
		{name: "password-word-not-assignment", input: "the password field is set in code"},
		{name: "passwd-short", input: "passwd=xy"},
		{name: "url-no-credentials", input: "https://example.com/path"},
		{name: "url-username-only", input: "https://user@example.com/path"},
	}

	for _, tt := range safe {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.Apply(tt.input)
			if err != nil {
				t.Fatalf("Apply() unexpected error: %v", err)
			}
			if got != tt.input {
				t.Errorf("Apply() modified safe text %q -> %q", tt.input, got)
			}
		})
	}
}

// TestCredentialFilter_README_MatchesPatterns ensures the README's redaction
// list does not drift from the code: every pattern id in credentialPatterns
// must be documented. This satisfies the R-06 "README matches the pattern
// table" DONE gate programmatically (in addition to the manual README update).
func TestCredentialFilter_README_MatchesPatterns(t *testing.T) {
	readme, err := os.ReadFile("../../../README.md")
	if err != nil {
		t.Fatalf("could not read README.md: %v", err)
	}
	doc := string(readme)

	for _, id := range filter.PatternIDs() {
		if !strings.Contains(doc, id) {
			t.Errorf("README.md does not document pattern id %q", id)
		}
	}
}
