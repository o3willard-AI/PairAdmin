package catalog

// Providers is the curated top-11 provider catalog. It is a package-level
// variable intended as read-only; the accessors below return copies so
// callers cannot mutate the canonical table.
var Providers = []Provider{
	// --- API providers (keyed) ---

	{
		ID:       "openai",
		Name:     "OpenAI",
		Adapter:  "openai",
		BaseURL:  "https://api.openai.com/v1",
		EnvKey:   "OPENAI_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "gpt-5.6-luna", Name: "gpt-5.6-luna", Reasoning: true, ToolCall: true},
			{ID: "gpt-5.6-sol", Name: "gpt-5.6-sol", Reasoning: true, ToolCall: true},
			{ID: "gpt-4.1", Name: "gpt-4.1", Reasoning: true, ToolCall: true},
			{ID: "gpt-4.1-mini", Name: "gpt-4.1-mini", Reasoning: true, ToolCall: true},
			{ID: "gpt-4o-mini", Name: "gpt-4o-mini", Reasoning: true, ToolCall: true},
		},
	},
	{
		ID:       "anthropic",
		Name:     "Anthropic",
		Adapter:  "anthropic",
		BaseURL:  "",
		EnvKey:   "ANTHROPIC_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "claude-sonnet-5", Name: "claude-sonnet-5", Reasoning: true, ToolCall: true},
			{ID: "claude-opus-5", Name: "claude-opus-5", Reasoning: true, ToolCall: true},
			{ID: "claude-haiku-4-5", Name: "claude-haiku-4-5", Reasoning: true, ToolCall: true},
		},
	},
	{
		ID:       "google",
		Name:     "Google Gemini",
		Adapter:  "gemini",
		BaseURL:  "https://generativelanguage.googleapis.com",
		EnvKey:   "GOOGLE_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "gemini-3.8-flash", Name: "gemini-3.8-flash", ToolCall: true},
			{ID: "gemini-3-flash", Name: "gemini-3-flash", ToolCall: true},
			{ID: "gemini-2.5-pro", Name: "gemini-2.5-pro", ToolCall: true},
			{ID: "gemini-2.5-flash", Name: "gemini-2.5-flash", ToolCall: true},
		},
	},
	{
		ID:       "deepseek",
		Name:     "DeepSeek",
		Adapter:  "openai",
		BaseURL:  "https://api.deepseek.com",
		EnvKey:   "DEEPSEEK_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "deepseek-v4-flash", Name: "deepseek-v4-flash", Reasoning: true},
			{ID: "deepseek-v4-pro", Name: "deepseek-v4-pro", Reasoning: true},
			{ID: "deepseek-v3.2", Name: "deepseek-v3.2", Reasoning: true},
		},
	},
	{
		ID:       "xai",
		Name:     "xAI (Grok)",
		Adapter:  "openai",
		BaseURL:  "https://api.x.ai/v1",
		EnvKey:   "XAI_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "grok-4.6", Name: "grok-4.6", ToolCall: true},
			{ID: "grok-4.5", Name: "grok-4.5", ToolCall: true},
		},
	},
	{
		ID:       "openrouter",
		Name:     "OpenRouter",
		Adapter:  "openai",
		BaseURL:  "https://openrouter.ai/api/v1",
		EnvKey:   "OPENROUTER_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "deepseek/deepseek-v4-flash", Name: "deepseek/deepseek-v4-flash", Reasoning: true, ToolCall: true},
			{ID: "google/gemini-3.8-flash", Name: "google/gemini-3.8-flash", Reasoning: true, ToolCall: true},
			{ID: "anthropic/claude-sonnet-5", Name: "anthropic/claude-sonnet-5", Reasoning: true, ToolCall: true},
			{ID: "z-ai/glm-5.3-flash", Name: "z-ai/glm-5.3-flash", Reasoning: true, ToolCall: true},
			{ID: "openai/gpt-5.6-luna", Name: "openai/gpt-5.6-luna", Reasoning: true, ToolCall: true},
		},
	},
	{
		ID:       "mistral",
		Name:     "Mistral",
		Adapter:  "openai",
		BaseURL:  "https://api.mistral.ai/v1",
		EnvKey:   "MISTRAL_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "mistral-large-latest", Name: "mistral-large-latest", ToolCall: true},
			{ID: "mistral-nemo", Name: "mistral-nemo", ToolCall: true},
			{ID: "mistral-small-latest", Name: "mistral-small-latest", ToolCall: true},
		},
	},
	{
		ID:       "groq",
		Name:     "Groq",
		Adapter:  "openai",
		BaseURL:  "https://api.groq.com/openai/v1",
		EnvKey:   "GROQ_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "llama-4-maverick", Name: "llama-4-maverick", ToolCall: true},
			{ID: "qwen3.5", Name: "qwen3.5", ToolCall: true},
		},
	},
	{
		ID:       "glm",
		Name:     "Z-AI (GLM)",
		Adapter:  "openai",
		BaseURL:  "https://open.bigmodel.cn/api/paas/v4",
		EnvKey:   "ZAI_API_KEY",
		NeedsKey: true,
		Models: []Model{
			{ID: "glm-5.3", Name: "glm-5.3", Reasoning: true},
			{ID: "glm-5.3-flash", Name: "glm-5.3-flash", Reasoning: true},
			{ID: "glm-5.2", Name: "glm-5.2", Reasoning: true},
		},
	},

	// --- Local providers (no key needed) ---

	{
		ID:       "ollama",
		Name:     "Ollama",
		Adapter:  "ollama",
		BaseURL:  "",
		EnvKey:   "",
		NeedsKey: false,
		Models:   nil, // local models are discovered at runtime, not cataloged
	},
	{
		ID:       "lmstudio",
		Name:     "LM Studio",
		Adapter:  "openai",
		BaseURL:  "http://localhost:1234/v1",
		EnvKey:   "",
		NeedsKey: false,
		Models:   nil, // local models are discovered at runtime, not cataloged
	},
}

// -----------------------------------------------------------------------------
// Read-only accessors
// -----------------------------------------------------------------------------

// GetProvider returns the Provider with the given ID and true, or a zero
// Provider and false if not found. The returned value is a copy.
func GetProvider(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// ListProviders returns a copy of the full provider list so callers cannot
// mutate the canonical Providers slice.
func ListProviders() []Provider {
	result := make([]Provider, len(Providers))
	copy(result, Providers)
	return result
}

// ModelsFor returns a copy of the models for the given provider ID, or nil
// if the provider is not found.
func ModelsFor(providerID string) []Model {
	for _, p := range Providers {
		if p.ID == providerID {
			models := make([]Model, len(p.Models))
			copy(models, p.Models)
			return models
		}
	}
	return nil
}
