// Package catalog provides a static, curated catalog of LLM providers and
// their supported models. The data is a release-time-refreshable snapshot —
// no network access, no sync, no mutation. Consumers (a future registry, UI
// picker, etc.) resolve providers and models through the read-only accessors
// in catalog.go.
//
// This is part of the PairAdmin LLM provider catalog effort (LLM-1).
package catalog

// Provider describes a single LLM provider entry in the catalog.
type Provider struct {
	ID       string  // stable id, e.g. "openai"
	Name     string  // display name, e.g. "OpenAI"
	Adapter  string  // which llm adapter serves it: "openai" | "anthropic" | "ollama" | "gemini"
	BaseURL  string  // for openai-compatible providers; "" for anthropic/ollama (n/a)
	EnvKey   string  // env var detected for the API key; "" when no key needed
	NeedsKey bool    // true => API key required (keychain/env); false => local (ollama/lmstudio)
	Models   []Model // models offered by this provider
}

// Model describes a single model offered by a Provider.
type Model struct {
	ID         string  // stable model identifier
	Name       string  // human-readable display name
	Context    int     // context window tokens; 0 = unknown
	Reasoning  bool    // supports reasoning/thinking
	ToolCall   bool    // supports tool/function calling
	InputCost  float64 // USD per 1M input tokens; 0 = unknown
	OutputCost float64 // USD per 1M output tokens; 0 = unknown
}
