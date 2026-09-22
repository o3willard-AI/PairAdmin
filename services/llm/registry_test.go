// Package llm - internal tests for the catalog-driven provider registry
// (ResolveProvider). Everything runs against the static catalog — no network.
package llm

import (
	"testing"
)

// keyedEnv sets every new-provider env var to a known value so env-fallback
// assertions are deterministic regardless of the host environment.
func keyedEnv(t *testing.T, value string) {
	t.Helper()
	for _, k := range []string{
		"GOOGLE_API_KEY", "DEEPSEEK_API_KEY", "XAI_API_KEY", "ZAI_API_KEY",
		"MISTRAL_API_KEY", "GROQ_API_KEY", "OPENAI_API_KEY",
	} {
		t.Setenv(k, value)
	}
}

func TestResolveProvider_AllElevenCatalogIDsResolve(t *testing.T) {
	for _, id := range []string{
		"openai", "anthropic", "google", "deepseek", "xai",
		"openrouter", "mistral", "groq", "glm", "ollama", "lmstudio",
	} {
		t.Run(id, func(t *testing.T) {
			keyedEnv(t, "env-key")
			p := ResolveProvider(ResolveConfig{Provider: id, Model: "m1"}, func(string) string { return "k" })
			if p == nil {
				t.Fatalf("expected a non-nil provider for catalog id %q", id)
			}
		})
	}
	// Mutation check: dropping any adapter case from ResolveProvider's
	// switch (or the catalog lookup) makes the corresponding subtest fail
	// with a nil provider.
}

func TestResolveProvider_AdapterTypeAndNamePerFamily(t *testing.T) {
	keyedEnv(t, "env-key")
	kf := func(string) string { return "k" }

	tests := []struct {
		id, wantName string
	}{
		{"openai", "openai"},
		{"anthropic", "anthropic"},
		{"ollama", "ollama"},
		{"google", "gemini"},
		// The openai-adapter set all report Name() "openai" (they share
		// the OpenAI-compatible wire protocol); their identity is asserted
		// via baseURL in the test below.
		{"deepseek", "openai"},
		{"xai", "openai"},
		{"openrouter", "openai"},
		{"mistral", "openai"},
		{"groq", "openai"},
		{"glm", "openai"},
		{"lmstudio", "openai"},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			p := ResolveProvider(ResolveConfig{Provider: tc.id, Model: "m1"}, kf)
			if p == nil {
				t.Fatalf("expected non-nil provider for %q", tc.id)
			}
			if p.Name() != tc.wantName {
				t.Errorf("%q: expected Name() %q, got %q", tc.id, tc.wantName, p.Name())
			}
		})
	}
}

func TestResolveProvider_OpenAIAdapterSetUsesCatalogBaseURLs(t *testing.T) {
	keyedEnv(t, "env-key")
	kf := func(string) string { return "k" }

	tests := []struct {
		id, wantBaseURL string
	}{
		{"openai", "https://api.openai.com/v1"},
		{"openrouter", "https://openrouter.ai/api/v1"},
		{"deepseek", "https://api.deepseek.com"},
		{"xai", "https://api.x.ai/v1"},
		{"mistral", "https://api.mistral.ai/v1"},
		{"groq", "https://api.groq.com/openai/v1"},
		{"glm", "https://open.bigmodel.cn/api/paas/v4"},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			p := ResolveProvider(ResolveConfig{Provider: tc.id, Model: "m1"}, kf)
			op, ok := p.(*OpenAIProvider)
			if !ok {
				t.Fatalf("%q: expected *OpenAIProvider, got %T", tc.id, p)
			}
			if op.baseURL != tc.wantBaseURL {
				t.Errorf("%q: expected baseURL %q, got %q", tc.id, tc.wantBaseURL, op.baseURL)
			}
			// Mutation check: ignoring the catalog's BaseURL (or falling
			// back to the old hardcoded "" for openai) fails this.
		})
	}
}

func TestResolveProvider_KeychainKeyWinsOverFallbacks(t *testing.T) {
	keyedEnv(t, "env-value")

	// keyFn provides a key for the new providers -> beats env fallback.
	for _, id := range []string{"google", "deepseek", "xai", "mistral", "groq", "glm"} {
		t.Run(id, func(t *testing.T) {
			p := ResolveProvider(ResolveConfig{Provider: id, Model: "m1"}, func(string) string { return "keychain-key" })
			if p == nil {
				t.Fatalf("expected non-nil provider")
			}
			got := ""
			switch v := p.(type) {
			case *OpenAIProvider:
				got = v.apiKey
			case *geminiProvider:
				got = v.apiKey
			default:
				t.Fatalf("unexpected provider type %T", p)
			}
			if got != "keychain-key" {
				t.Errorf("%q: expected keychain key, got %q", id, got)
			}
		})
	}

	// keyFn wins for the legacy five too (Config-field fallback loses).
	p := ResolveProvider(ResolveConfig{
		Provider: "openai", Model: "m", OpenAIKey: "config-field-key",
	}, func(string) string { return "keychain-key" })
	if op := p.(*OpenAIProvider); op.apiKey != "keychain-key" {
		t.Errorf("expected keychain key to win over Config field, got %q", op.apiKey)
	}
	p = ResolveProvider(ResolveConfig{
		Provider: "anthropic", Model: "m", AnthropicKey: "config-field-key",
	}, func(string) string { return "keychain-key" })
	if ap := p.(*AnthropicProvider); ap.apiKey != "keychain-key" {
		t.Errorf("anthropic: expected keychain key, got %q", ap.apiKey)
	}
	// Mutation check: swapping the precedence (Config field checked before
	// keyFn) makes these fail — keyFn-first is load-bearing.
}

func TestResolveProvider_NewProvidersFallBackToEnvVar(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-env-key")
	t.Setenv("GOOGLE_API_KEY", "google-env-key")

	p := ResolveProvider(ResolveConfig{Provider: "deepseek", Model: "m1"}, func(string) string { return "" })
	if op, ok := p.(*OpenAIProvider); !ok {
		t.Fatalf("expected *OpenAIProvider, got %T", p)
	} else if op.apiKey != "deepseek-env-key" {
		t.Errorf("expected env fallback DEEPSEEK_API_KEY, got %q", op.apiKey)
	}

	p = ResolveProvider(ResolveConfig{Provider: "google", Model: "m1"}, func(string) string { return "" })
	if gp, ok := p.(*geminiProvider); !ok {
		t.Fatalf("expected *geminiProvider, got %T", p)
	} else if gp.apiKey != "google-env-key" {
		t.Errorf("expected env fallback GOOGLE_API_KEY, got %q", gp.apiKey)
	}
	// Mutation check: resolving new providers' keys from the legacy Config
	// fields instead of os.Getenv(p.EnvKey) leaves them empty and fails this.
}

func TestResolveProvider_LegacyFallbacksPreserved(t *testing.T) {
	kf := func(string) string { return "" }

	// openrouter's legacy chain: OpenRouterKey empty -> OpenAIKey.
	p := ResolveProvider(ResolveConfig{
		Provider: "openrouter", Model: "m", OpenAIKey: "shared-openai-key",
	}, kf)
	op := p.(*OpenAIProvider)
	if op.apiKey != "shared-openai-key" {
		t.Errorf("openrouter should fall back to OpenAIKey, got %q", op.apiKey)
	}
	// And OpenRouterKey itself when set.
	p = ResolveProvider(ResolveConfig{
		Provider: "openrouter", Model: "m", OpenRouterKey: "or-key",
	}, kf)
	if p.(*OpenAIProvider).apiKey != "or-key" {
		t.Error("openrouter should prefer OpenRouterKey when set")
	}

	// ollama: optional key still resolves from keyFn (remote instances).
	p = ResolveProvider(ResolveConfig{Provider: "ollama", Model: "m"}, func(string) string { return "ollama-bearer" })
	if o := p.(*ollamaProvider); o.token != "ollama-bearer" {
		t.Errorf("ollama token expected from keyFn, got %q", o.token)
	}
	// Mutation check: dropping the legacy fallback branches (openrouter's
	// OpenAIKey fallback, ollama's keyFn) changes these results.
}

func TestResolveProvider_LMStudioHostResolution(t *testing.T) {
	p := ResolveProvider(ResolveConfig{Provider: "lmstudio", Model: "m1", LMStudioHost: "http://host.docker.internal:1234/v1"}, func(string) string { return "" })
	op, ok := p.(*OpenAIProvider)
	if !ok {
		t.Fatalf("expected *OpenAIProvider, got %T", p)
	}
	if op.baseURL != "http://host.docker.internal:1234/v1" {
		t.Errorf("expected cfg.LMStudioHost to win, got %q", op.baseURL)
	}

	p = ResolveProvider(ResolveConfig{Provider: "lmstudio", Model: "m1"}, func(string) string { return "" })
	op = p.(*OpenAIProvider)
	if op.baseURL != "http://localhost:1234/v1" {
		t.Errorf("expected default localhost base URL, got %q", op.baseURL)
	}
	if op.apiKey != "" {
		t.Errorf("lmstudio is keyless, got %q", op.apiKey)
	}
	// Mutation check: using the catalog BaseURL directly (skipping the
	// LMStudioHost-with-default branch) yields the same default host but
	// breaks custom-host resolution — the first assertion is load-bearing.
}

func TestResolveProvider_DisabledEmptyUnknownReturnNil(t *testing.T) {
	keyedEnv(t, "env-key")
	kf := func(string) string { return "k" }

	for _, id := range []string{"", "disabled", "no-such-provider"} {
		if p := ResolveProvider(ResolveConfig{Provider: id, Model: "m1"}, kf); p != nil {
			t.Errorf("provider %q must resolve to nil, got %v", id, p)
		}
	}
	// Mutation check: constructing a provider for unknown ids (or dropping
	// the "disabled"/empty special cases) returns non-nil and fails this.
}

func TestResolveProvider_OllamaBadHostReturnsNil(t *testing.T) {
	// NewOllamaProvider validates the host; an invalid scheme must surface
	// as nil, not a broken provider — same semantics as the old switch.
	p := ResolveProvider(ResolveConfig{Provider: "ollama", Model: "m", OllamaHost: "ftp://not-allowed"}, func(string) string { return "" })
	if p != nil {
		t.Errorf("expected nil provider for an invalid ollama host, got %T", p)
	}
	// Mutation check: ignoring NewOllamaProvider's error (returning the
	// zero provider anyway) makes this fail.
}
