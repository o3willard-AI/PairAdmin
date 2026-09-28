package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pairadmin/services/llm/catalog"
	"pairadmin/services/llm/policy"
)

// isolateConfig points the app config dir at a temp HOME and writes a config
// with the given enabled/disabled provider lists, so EffectiveBundle() reads
// them exactly as a hand-edited config.yaml would be read.
func isolateConfig(t *testing.T, enabled, disabled []string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)

	var b strings.Builder
	b.WriteString("scanner_enabled: false\n")
	if len(enabled) > 0 {
		b.WriteString("enabled_providers:\n")
		for _, p := range enabled {
			b.WriteString("  - " + p + "\n")
		}
	}
	if len(disabled) > 0 {
		b.WriteString("disabled_providers:\n")
		for _, p := range disabled {
			b.WriteString("  - " + p + "\n")
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, ".pairadmin"), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".pairadmin", "config.yaml"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func providerIDs(ps []catalog.Provider) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.ID)
	}
	return out
}

func hasID(ps []catalog.Provider, id string) bool {
	for _, p := range ps {
		if p.ID == id {
			return true
		}
	}
	return false
}

func modelIDs(ps []catalog.Provider, providerID string) []string {
	for _, p := range ps {
		if p.ID == providerID {
			out := make([]string, 0, len(p.Models))
			for _, m := range p.Models {
				out = append(out, m.ID)
			}
			return out
		}
	}
	return nil
}

// TestFilterCatalog_DisabledProviderIsRemoved pins enforcement point 1: a
// denied provider never reaches the catalog at all, and everything else
// survives untouched.
//
// Mutation check: removing the `continue` for a denied provider in
// filterCatalogByPolicy (i.e. appending it anyway) makes this test FAIL on
// "openai is still present".
func TestFilterCatalog_DisabledProviderIsRemoved(t *testing.T) {
	isolateConfig(t, nil, []string{"openai"})

	filtered := FilterCatalog(catalog.ListProviders())
	if hasID(filtered, "openai") {
		t.Error("disabled provider openai is still in the filtered catalog")
	}
	// The rest of the catalog is intact — this narrows, it does not truncate.
	for _, id := range []string{"anthropic", "google", "ollama", "lmstudio", "groq"} {
		if !hasID(filtered, id) {
			t.Errorf("unrelated provider %q was removed; the filter must only narrow", id)
		}
	}
	// The canonical catalog table is not mutated by filtering.
	if !hasID(catalog.ListProviders(), "openai") {
		t.Error("filtering mutated the canonical catalog table")
	}
}

// TestFilterCatalog_EnabledListActsAsAllowlist pins that a non-empty
// enabled_providers inverts the catalog to just those providers, and that each
// allowed provider keeps its models (so the picker is still usable).
//
// Mutation check: dropping the `filtered.Models = models` assignment makes
// the model-list assertion FAIL only if it is also dropping models — the
// load-bearing half is removing the provider-level `continue`, which makes the
// "anthropic removed" assertion FAIL.
func TestFilterCatalog_EnabledListActsAsAllowlist(t *testing.T) {
	isolateConfig(t, []string{"openai", "anthropic"}, nil)

	filtered := FilterCatalog(catalog.ListProviders())
	got := providerIDs(filtered)
	if len(got) != 2 {
		t.Fatalf("allowlist: got %v, want exactly [openai anthropic]", got)
	}
	if !hasID(filtered, "openai") || !hasID(filtered, "anthropic") {
		t.Errorf("allowlist dropped a listed provider: %v", got)
	}
	if hasID(filtered, "google") {
		t.Error("allowlist did not restrict an unlisted provider")
	}
	// Models survive for the allowed providers, or the picker would be empty.
	if n := len(modelIDs(filtered, "openai")); n == 0 {
		t.Error("allowlisted provider lost all its models — the picker would be unusable")
	}
}

// TestFilterCatalog_ModelLevelDeny pins model.use filtering: a provider can
// stay while individual models are removed.
//
// Mutation check: removing the model `continue` in filterCatalogByPolicy makes
// the "denied model still listed" assertion FAIL.
func TestFilterCatalog_ModelLevelDeny(t *testing.T) {
	// Disable a provider's whole model set except one, by denying the
	// provider's models via a model.use pattern on one specific model is not
	// expressible through the config lists, so this exercises the engine
	// directly through the seam override below.
	orig := enforcePolicy
	defer func() { enforcePolicy = orig }()

	enforcePolicy = func(_ *policy.PolicyBundle, providers []catalog.Provider) []catalog.Provider {
		b := &policy.PolicyBundle{
			SchemaVersion: policy.CurrentSchemaVersion,
			DefaultAllow:  "allow",
			Statements: []policy.PolicyStatement{
				{Action: policy.ActionModelUse, Effect: "deny", Resource: "openai/gpt-4o-mini"},
			},
		}
		return filterCatalogByPolicy(b, providers)
	}

	filtered := FilterCatalog(catalog.ListProviders())
	if !hasID(filtered, "openai") {
		t.Fatal("a model-level deny must not remove the provider itself")
	}
	models := modelIDs(filtered, "openai")
	for _, m := range models {
		if m == "gpt-4o-mini" {
			t.Error("denied model gpt-4o-mini is still in the catalog")
		}
	}
	// Sanity: the model really was there to begin with, so this is not
	// passing because the fixture is empty.
	if len(models) == 0 {
		t.Fatal("fixture has no models; the test cannot detect anything")
	}
	found := false
	for _, m := range models {
		if m == "gpt-5.6-luna" {
			found = true
		}
	}
	if !found {
		t.Error("an allowed model was removed too; the filter over-narrows")
	}
}

// TestEnforcePolicySeamIsOverridable pins the fork contract: the seam is a
// package-level var a downstream fork can replace, and restoring the default
// afterwards leaves the OSS behavior intact. This is the same shape as
// services/scanner's scanAllowed.
//
// Mutation check: changing `var enforcePolicy = filterCatalogByPolicy` to a
// non-var (a plain function) makes this test fail to COMPILE, which is the
// intended outcome — the fork could no longer hook in.
func TestEnforcePolicySeamIsOverridable(t *testing.T) {
	orig := enforcePolicy
	defer func() {
		enforcePolicy = orig
		// The restored default must be the OSS behavior again.
		if enforcePolicy == nil {
			t.Fatal("seam was not restored")
		}
		isolateConfig(t, nil, []string{"openai"})
		if hasID(FilterCatalog(catalog.ListProviders()), "openai") {
			t.Error("restored default seam does not enforce the OSS self-restriction")
		}
	}()

	calls := 0
	enforcePolicy = func(b *policy.PolicyBundle, providers []catalog.Provider) []catalog.Provider {
		calls++
		// A fork enforcing a signed bundle would filter by signature here.
		if b == nil {
			t.Error("fork seam received a nil bundle")
		}
		return nil // maximally restrictive, as a deny-all fork would
	}
	if got := FilterCatalog(catalog.ListProviders()); len(got) != 0 {
		t.Errorf("overridden seam was not consulted: got %d providers", len(got))
	}
	if calls != 1 {
		t.Errorf("seam called %d times, want 1", calls)
	}
}

// TestFilterCatalog_NilSeamIsPassthrough pins the nil-seam guard: a fork that
// sets the seam to nil gets the unfiltered list rather than a panic or a
// total lockout.
func TestFilterCatalog_NilSeamIsPassthrough(t *testing.T) {
	orig := enforcePolicy
	defer func() { enforcePolicy = orig }()
	enforcePolicy = nil

	isolateConfig(t, nil, []string{"openai"}) // a denylist is configured...
	got := FilterCatalog(catalog.ListProviders())
	// ...but a nil seam must not pretend to enforce it.
	if !hasID(got, "openai") {
		t.Error("nil seam should return the input unchanged")
	}
}

// TestCheckRequest_DeniesHandEditedConfig pins enforcement point 2: the
// request-time re-check reads the config on every call, so a config edited
// after startup is honored immediately — this is what makes the re-check more
// than a duplicate of the catalog filter.
//
// Mutation check: replacing the body of CheckRequest with `return nil` makes
// every deny sub-case FAIL.
func TestCheckRequest_DeniesHandEditedConfig(t *testing.T) {
	isolateConfig(t, nil, nil)

	// Unrestricted: allowed.
	if err := CheckRequest("openai", "gpt-4.1"); err != nil {
		t.Fatalf("unconfigured policy denied a request: %v", err)
	}

	// Hand-edit the config after the service would have started.
	isolateConfig(t, nil, []string{"openai"})

	if err := CheckRequest("openai", "gpt-4.1"); err == nil {
		t.Error("request-time re-check allowed a provider disabled by the config")
	} else if !strings.Contains(err.Error(), "openai") {
		t.Errorf("error must name the refused resource, got: %v", err)
	}
	// Other providers still work: the check narrows only.
	if err := CheckRequest("anthropic", "claude-sonnet-5"); err != nil {
		t.Errorf("unrelated provider was denied: %v", err)
	}
}

// TestCheckRequest_AllowlistDeniesUnlistedProvider pins what the OSS config
// lists can actually express: they are provider-granular, so an unlisted
// provider is refused at the PROVIDER level and the error names the provider.
// The model.use branch is pinned separately by
// TestCheckRequestAgainst_ModelLevelDeny, which injects a bundle directly
// because no OSS config field can express a model-level deny today.
func TestCheckRequest_AllowlistDeniesUnlistedProvider(t *testing.T) {
	isolateConfig(t, []string{"openai"}, nil)

	// An allowlisted provider and its own models are permitted.
	if err := CheckRequest("openai", "gpt-4.1"); err != nil {
		t.Errorf("allowlisted provider's own model denied: %v", err)
	}
	// An unlisted provider is refused, and the error names it.
	if err := CheckRequest("deepseek", "deepseek-v4-pro"); err == nil {
		t.Error("allowlist did not deny an unlisted provider at request time")
	} else if !strings.Contains(err.Error(), "deepseek") {
		t.Errorf("error should name the refused provider, got: %v", err)
	}
}

// TestCheckRequestAgainst_ModelLevelDeny pins the model.use branch of the
// request-time re-check: a provider that IS allowed can still have an
// individual model refused. This is the case a model typed straight into the
// UI would hit, and the catalog filter alone does not cover it.
//
// Mutation check: deleting the `if model != ""` block (the model.use
// evaluation) from checkRequestAgainst makes the "denied model still allowed"
// sub-case FAIL.
func TestCheckRequestAgainst_ModelLevelDeny(t *testing.T) {
	b := &policy.PolicyBundle{
		SchemaVersion: policy.CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements: []policy.PolicyStatement{
			{Action: policy.ActionModelUse, Effect: "deny", Resource: "openai/gpt-4o-mini"},
		},
	}
	// The provider is fine...
	if err := checkRequestAgainst(b, "openai", "gpt-4.1"); err != nil {
		t.Errorf("allowed model refused: %v", err)
	}
	// ...and the denied one is refused, naming provider/model.
	if err := checkRequestAgainst(b, "openai", "gpt-4o-mini"); err == nil {
		t.Error("model-level deny was not enforced at request time")
	} else if !strings.Contains(err.Error(), "openai/gpt-4o-mini") {
		t.Errorf("error should name the provider/model resource, got: %v", err)
	}
	// With no model configured the model check is skipped, not mis-evaluated.
	if err := checkRequestAgainst(b, "openai", ""); err != nil {
		t.Errorf("model-less request denied by a model.use rule: %v", err)
	}
}

// TestCheckRequest_OptOutAndEmptyModel pins the two non-policy paths: the
// existing "disabled" opt-out is not treated as a policy denial, and a
// provider with no model configured still gets its provider.use check without
// evaluating a bare id as a model.
func TestCheckRequest_OptOutAndEmptyModel(t *testing.T) {
	isolateConfig(t, []string{"openai"}, nil)

	// "disabled" is the pre-existing opt-out, handled before the policy.
	if err := CheckRequest("disabled", ""); err != nil {
		t.Errorf(`"disabled" must not be reported as a policy denial: %v`, err)
	}
	if err := CheckRequest("", ""); err != nil {
		t.Errorf("empty provider must not be reported as a policy denial: %v", err)
	}
	// A model-less request is still checked against provider.use.
	if err := CheckRequest("anthropic", ""); err == nil {
		t.Error("model-less request skipped the provider.use check")
	}
	if err := CheckRequest("openai", ""); err != nil {
		t.Errorf("allowed provider denied with no model configured: %v", err)
	}
}

// TestEffectiveBundle_DefaultsToUnrestricted pins the no-config path: a
// missing or unreadable config must NOT lock the user out of every provider.
func TestEffectiveBundle_DefaultsToUnrestricted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	// No config file at all.

	if err := CheckRequest("openai", "gpt-4.1"); err != nil {
		t.Errorf("missing config must not restrict anything: %v", err)
	}
	if got := len(FilterCatalog(catalog.ListProviders())); got != len(catalog.Providers) {
		t.Errorf("missing config changed the catalog size: got %d, want %d", got, len(catalog.Providers))
	}
}
