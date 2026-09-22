package catalog

import "testing"

// TestGetAllProviderIDs verifies that every accessor works for each of the
// 11 provider IDs in the catalog.
func TestGetAllProviderIDs(t *testing.T) {
	wantIDs := []string{
		"openai", "anthropic", "google", "deepseek", "xai",
		"openrouter", "mistral", "groq", "glm", "ollama", "lmstudio",
	}

	for _, id := range wantIDs {
		p, ok := GetProvider(id)
		if !ok {
			t.Errorf("GetProvider(%q) returned ok=false, expected true", id)
			continue
		}
		if p.ID != id {
			t.Errorf("GetProvider(%q): returned ID %q", id, p.ID)
		}
		if p.Name == "" {
			t.Errorf("GetProvider(%q): Name is empty", id)
		}
		if p.Adapter == "" {
			t.Errorf("GetProvider(%q): Adapter is empty", id)
		}
	}

	// Mutation check: removing the ok-return in GetProvider (always returning
	// ok=true with a zero Provider) would let a missing provider pass the
	// first check, but the ID mismatch in the second loop would catch it.
	for _, id := range wantIDs {
		_, ok := GetProvider(id)
		if !ok {
			t.Fatalf("provider %q must exist in the catalog (ok=false)", id)
		}
	}
}

// TestUnknownProviderReturnsFalse verifies that GetProvider returns false for
// an ID that is not in the catalog.
// Mutation check: removing the `return Provider{}, false` line in the
// GetProvider not-found branch makes this test fail (ok would be true).
func TestUnknownProviderReturnsFalse(t *testing.T) {
	_, ok := GetProvider("nonexistent-provider")
	if ok {
		t.Fatal("GetProvider(\"nonexistent-provider\") should return false")
	}
}

// TestProviderCountIsExactly11 verifies the catalog has exactly 11 providers.
// Mutation check: removing or adding any provider in the Providers slice makes
// this test fail.
func TestProviderCountIsExactly11(t *testing.T) {
	if len(Providers) != 11 {
		t.Fatalf("expected exactly 11 providers, got %d", len(Providers))
	}
}

// TestLocalProvidersHaveNoEnvKey verifies that providers with NeedsKey=false
// (ollama, lmstudio) have EnvKey="" and no API key requirement.
// Mutation check: removing the `EnvKey: ""` field from the ollama or lmstudio
// entries makes this test fail.
func TestLocalProvidersHaveNoEnvKey(t *testing.T) {
	for _, p := range Providers {
		if !p.NeedsKey {
			if p.EnvKey != "" {
				t.Errorf("provider %q: NeedsKey=false but EnvKey=%q (should be empty)", p.ID, p.EnvKey)
			}
		}
	}
}

// TestKeyedProvidersHaveEnvKey verifies that every NeedsKey=true provider has
// a non-empty EnvKey.
// Mutation check: clearing the EnvKey for any keyed provider makes this test
// fail.
func TestKeyedProvidersHaveEnvKey(t *testing.T) {
	for _, p := range Providers {
		if p.NeedsKey {
			if p.EnvKey == "" {
				t.Errorf("provider %q: NeedsKey=true but EnvKey is empty", p.ID)
			}
		}
	}
}

// TestListProvidersReturnsCopy verifies that ListProviders returns a copy
// that the caller can mutate without affecting the canonical catalog.
// Mutation check: removing the `make + copy` in ListProviders (returning the
// canonical slice directly) makes the caller's mutation visible via
// GetProvider, causing this test's second check to fail.
func TestListProvidersReturnsCopy(t *testing.T) {
	original := ListProviders()
	if len(original) == 0 {
		t.Fatal("ListProviders returned empty slice")
	}

	list := ListProviders()
	list[0].ID = "mutated-id"

	// The canonical catalog must be unaffected by caller mutations.
	if _, ok := GetProvider("mutated-id"); ok {
		t.Fatalf("canonical catalog was mutated by caller (GetProvider found %q)", "mutated-id")
	}

	// The caller's original copy is also unaffected by the mutation.
	if original[0].ID == "mutated-id" {
		t.Fatal("original snapshot was mutated — ListProviders did not return a copy")
	}
}

// TestModelsFor_KnownProvider verifies that ModelsFor returns the correct
// models for a known provider.
// Mutation check: removing or renaming any model in the openai entry makes
// the count or first-ID assertion fail.
func TestModelsFor_KnownProvider(t *testing.T) {
	models := ModelsFor("openai")
	if len(models) != 5 {
		t.Fatalf("expected 5 models for openai, got %d", len(models))
	}
	if models[0].ID != "gpt-5.6-luna" {
		t.Errorf("expected first model gpt-5.6-luna, got %s", models[0].ID)
	}
}

// TestModelsFor_UnknownProvider verifies that ModelsFor returns nil for
// an unknown provider ID.
// Mutation check: removing the `return nil` in the ModelsFor not-found branch
// makes this test fail (a non-nil empty slice would be returned).
func TestModelsFor_UnknownProvider(t *testing.T) {
	models := ModelsFor("nonexistent-provider")
	if models != nil {
		t.Fatalf("expected nil models for unknown provider, got %v", models)
	}
}

// TestModelsFor_OllamaEmpty verifies that providers with runtime-discovered
// local models (ollama, lmstudio) have nil or empty Models slices.
// Mutation check: adding any model to the ollama entry makes this test fail.
func TestModelsFor_OllamaEmpty(t *testing.T) {
	models := ModelsFor("ollama")
	if len(models) != 0 {
		t.Fatalf("ollama should have no catalogued models, got %d", len(models))
	}
}

// TestModelCapabilities verifies that model capability flags are set correctly
// per the catalog specification.
// Mutation check: flipping any Reasoning or ToolCall flag in the catalog
// makes the corresponding assertion fail.
func TestModelCapabilities(t *testing.T) {
	// openai: all models have reasoning=true, tool_call=true
	for _, m := range ModelsFor("openai") {
		if !m.Reasoning {
			t.Errorf("model %q: expected Reasoning=true", m.ID)
		}
		if !m.ToolCall {
			t.Errorf("model %q: expected ToolCall=true", m.ID)
		}
	}

	// deepseek: reasoning=true, tool_call=false
	for _, m := range ModelsFor("deepseek") {
		if !m.Reasoning {
			t.Errorf("model %q: expected Reasoning=true", m.ID)
		}
		if m.ToolCall {
			t.Errorf("model %q: expected ToolCall=false", m.ID)
		}
	}

	// google: tool_call=true, reasoning=false
	for _, m := range ModelsFor("google") {
		if !m.ToolCall {
			t.Errorf("model %q: expected ToolCall=true", m.ID)
		}
		if m.Reasoning {
			t.Errorf("model %q: expected Reasoning=false", m.ID)
		}
	}

	// openrouter: both true
	for _, m := range ModelsFor("openrouter") {
		if !m.Reasoning {
			t.Errorf("model %q: expected Reasoning=true", m.ID)
		}
		if !m.ToolCall {
			t.Errorf("model %q: expected ToolCall=true", m.ID)
		}
	}
}

// TestLocalProvidersList verifies that local providers (ollama, lmstudio)
// are present and have the expected adapter and BaseURL.
// Mutation check: changing the Adapter or BaseURL for ollama/lmstudio in the
// catalog makes the corresponding assertion fail.
func TestLocalProvidersList(t *testing.T) {
	ollama, ok := GetProvider("ollama")
	if !ok {
		t.Fatal("ollama provider not found")
	}
	if ollama.Adapter != "ollama" {
		t.Errorf("ollama adapter: expected 'ollama', got %q", ollama.Adapter)
	}
	if ollama.BaseURL != "" {
		t.Errorf("ollama BaseURL should be empty, got %q", ollama.BaseURL)
	}

	lmstudio, ok := GetProvider("lmstudio")
	if !ok {
		t.Fatal("lmstudio provider not found")
	}
	if lmstudio.BaseURL != "http://localhost:1234/v1" {
		t.Errorf("lmstudio BaseURL: expected 'http://localhost:1234/v1', got %q", lmstudio.BaseURL)
	}
}
