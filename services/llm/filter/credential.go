package filter

import (
	"fmt"
	"regexp"
)

// credPattern holds a named regex credential pattern.
type credPattern struct {
	id string
	re *regexp.Regexp
}

// PatternIDs returns the ids of all configured credential patterns, so
// callers (and the README-drift test) can enumerate which rules exist.
func PatternIDs() []string {
	out := make([]string, 0, len(credentialPatterns))
	for _, p := range credentialPatterns {
		out = append(out, p.id)
	}
	return out
}

// credentialPatterns are fallback regex patterns for common credential formats.
// These are compiled once at startup and always applied regardless of whether
// a gitleaks detector is available.
var credentialPatterns = []*credPattern{
	{
		id: "aws-access-key-id",
		re: regexp.MustCompile(`(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`),
	},
	{
		id: "github-token",
		re: regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}|github_pat_[0-9a-zA-Z_]{82}`),
	},
	{
		id: "openai-api-key",
		re: regexp.MustCompile(`sk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}|sk-proj-[a-zA-Z0-9\-_]{100,}`),
	},
	{
		id: "anthropic-api-key",
		re: regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{95,}`),
	},
	{
		id: "bearer-token",
		re: regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-\._~\+\/]{20,}=*`),
	},
	{
		id: "generic-api-key",
		re: regexp.MustCompile(`(?i)(api[_-]?key|apikey|api[_-]?secret)\s*[:=]\s*['"]?[A-Za-z0-9\-_]{16,}['"]?`),
	},
	{
		id: "gitlab-personal-access-token",
		re: regexp.MustCompile(`\bglpat-[0-9a-zA-Z_\-]{20,}`),
	},
	{
		id: "slack-token",
		re: regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}`),
	},
	{
		id: "google-api-key",
		re: regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	},
	{
		id: "google-service-account",
		re: regexp.MustCompile(`"type"\s*:\s*"service_account"[\s\S]*?"private_key"\s*:\s*"[^"]*"`),
	},
	{
		id: "azure-account-key",
		re: regexp.MustCompile(`\b(?:AccountKey|SharedAccessKey)\s*=\s*[A-Za-z0-9+/]{40,}={0,2}`),
	},
	{
		id: "jwt",
		re: regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{5,}\.[A-Za-z0-9_\-]{5,}\.[A-Za-z0-9_\-]{5,}={0,2}\b`),
	},
	{
		id: "pem-private-key",
		re: regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`),
	},
	{
		id: "password-assignment",
		re: regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])(?:password|passwd)\s*[:=]\s*[^[:space:]'";]{6,}`),
	},
	{
		id: "connection-string-credentials",
		re: regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://[^/\s:@]+:[^@\s/]+@`),
	},
}

// CredentialFilter detects and redacts credential patterns in content.
// It uses regex fallback patterns for common credential formats (AWS keys,
// GitHub/GitLab tokens, OpenAI/Anthropic keys, Slack tokens, Google API keys
// & service-account JSON, Azure storage keys, bearer tokens & bare JWTs, PEM
// private keys, password/passwd assignments, and connection strings with
// embedded credentials).
type CredentialFilter struct{}

// NewCredentialFilter creates a new CredentialFilter.
// Note: gitleaks library is not currently a dependency. Regex fallback patterns
// are used instead. The gitleaks integration would require adding
// github.com/zricethezav/gitleaks/v8 as a dependency.
func NewCredentialFilter() (*CredentialFilter, error) {
	return &CredentialFilter{}, nil
}

// Apply scans content for credential patterns and replaces each match with
// [REDACTED:<rule_id>]. All configured patterns are applied.
func (f *CredentialFilter) Apply(content string) (string, error) {
	result := content
	for _, pattern := range credentialPatterns {
		id := pattern.id
		result = pattern.re.ReplaceAllStringFunc(result, func(match string) string {
			return fmt.Sprintf("[REDACTED:%s]", id)
		})
	}
	return result, nil
}
