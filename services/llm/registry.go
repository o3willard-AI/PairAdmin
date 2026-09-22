// Package llm — catalog-driven provider registry.
//
// ResolveProvider replaces the old hardcoded buildProvider switch: every
// provider in the static catalog (services/llm/catalog) resolves through one
// path, so new catalog entries work without touching this file again. The
// original five providers (openai, anthropic, ollama, openrouter, lmstudio)
// keep their EXACT legacy behavior — key-resolution order, baseURL handling,
// and the "disabled" opt-out are preserved bit-for-bit.
package llm

import (
	"os"

	"pairadmin/services/llm/catalog"
)

// ResolveConfig carries everything resolution needs from the app's LLM
// Config (services.Config). It is a mirror struct, not the real Config,
// because package llm must never import services (import cycle).
type ResolveConfig struct {
	// Provider is the catalog provider id, or "disabled"/"" for none.
	Provider string
	// Model is the model id passed to the adapter.
	Model string
	// Legacy env-var/Config-field key fallbacks for the original five
	// providers — preserved exactly (openrouter additionally falls back to
	// OpenAIKey).
	OpenAIKey     string
	AnthropicKey  string
	OpenRouterKey string
	OllamaKey     string
	// Local-instance hosts.
	OllamaHost   string
	LMStudioHost string
}

// ResolveProvider binds a catalog provider id to a live Provider.
//
// Resolution order:
//  1. "" and "disabled" resolve to nil (never construct, never dial) — the
//     same opt-out semantics as the old switch's disabled/default cases.
//  2. Unknown catalog ids resolve to nil.
//  3. The API key comes from keyFn(p.ID) (the keychain Enclave) first; when
//     empty, the legacy five fall back to their Config field and the new
//     providers to os.Getenv(p.EnvKey). Keyless providers (lmstudio)
//     resolve no key at all.
//  4. OpenAI-adapter providers use the catalog BaseURL, except lmstudio
//     (cfg.LMStudioHost, default http://localhost:1234/v1). Ollama uses
//     cfg.OllamaHost. Anthropic and Gemini have no configurable base.
func ResolveProvider(cfg ResolveConfig, keyFn func(string) string) Provider {
	switch cfg.Provider {
	case "", "disabled":
		// Explicit opt-out (Settings → LLM Config "Disable Pair LLM") or no
		// provider configured: never construct a provider, so SendMessage
		// can never attempt any connection.
		return nil
	}

	p, ok := catalog.GetProvider(cfg.Provider)
	if !ok {
		return nil // unknown provider: nothing to build
	}

	key := resolveKey(p, cfg, keyFn)

	switch p.Adapter {
	case "openai":
		baseURL := p.BaseURL
		if p.ID == "lmstudio" {
			// LM Studio is the one openai-adapter provider whose host is
			// user-configurable (LMSTUDIO_HOST / Settings); the catalog URL
			// is only its default.
			baseURL = cfg.LMStudioHost
			if baseURL == "" {
				baseURL = "http://localhost:1234/v1"
			}
		}
		return NewOpenAIProvider(key, baseURL, cfg.Model)
	case "anthropic":
		return NewAnthropicProvider(key, cfg.Model)
	case "ollama":
		// Host validation lives in NewOllamaProvider; an invalid host is a
		// runtime issue, not a broken provider — nil (same as the old
		// switch's error path).
		op, err := NewOllamaProvider(key, cfg.OllamaHost, cfg.Model)
		if err != nil {
			return nil
		}
		return op
	case "gemini":
		return NewGeminiProvider(key, cfg.Model)
	}
	return nil // unknown adapter (future catalog entries): fail safe
}

// resolveKey applies the key-resolution order for one catalog provider:
// keyFn(p.ID) first (keychain), then the legacy-five Config-field fallbacks,
// then os.Getenv(p.EnvKey) for the new providers. LM Studio is keyless.
func resolveKey(p catalog.Provider, cfg ResolveConfig, keyFn func(string) string) string {
	if p.ID == "lmstudio" {
		return "" // keyless local adapter (the old switch passed "" directly)
	}
	key := ""
	if keyFn != nil {
		key = keyFn(p.ID)
	}
	if key != "" {
		return key
	}
	switch p.ID {
	// The legacy five keep their exact Config-field fallbacks so existing
	// behavior (and tests) is unchanged.
	case "openai":
		return cfg.OpenAIKey
	case "anthropic":
		return cfg.AnthropicKey
	case "openrouter":
		if cfg.OpenRouterKey != "" {
			return cfg.OpenRouterKey
		}
		return cfg.OpenAIKey // preserved legacy fallback
	case "ollama":
		// Optional bearer token for authenticated remote Ollama servers;
		// local instances need none.
		return cfg.OllamaKey
	default:
		// New providers (google, deepseek, xai, mistral, groq, glm): the
		// env var named by the catalog entry, e.g. DEEPSEEK_API_KEY.
		if p.EnvKey == "" {
			return ""
		}
		return os.Getenv(p.EnvKey)
	}
}
