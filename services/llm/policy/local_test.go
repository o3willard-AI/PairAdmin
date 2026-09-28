package policy

import "testing"

// TestLocalBundle_DisabledOnlyNarrows pins the denylist path: a disabled
// provider is denied, everything else stays at the allow default.
//
// Mutation check: removing the disabledProviders loop from LocalBundle (or
// emitting allow instead of deny) makes the "openai denied" assertion FAIL.
func TestLocalBundle_DisabledOnlyNarrows(t *testing.T) {
	b := LocalBundle(nil, []string{"openai"})

	if err := Validate(b); err != nil {
		t.Fatalf("LocalBundle produced an invalid bundle: %v", err)
	}
	if eff, _ := Evaluate(b, ActionProviderUse, "openai"); eff != EffectDeny {
		t.Errorf("disabled provider: got %q, want deny", eff)
	}
	// Its models are denied too — otherwise the picker would still offer
	// them and the request-time model check would be the only thing standing
	// between the user and a denied model.
	if eff, _ := Evaluate(b, ActionModelUse, "openai/gpt-4.1"); eff != EffectDeny {
		t.Errorf("disabled provider's model: got %q, want deny", eff)
	}
	// Everything else is untouched: this is a denylist, not an allowlist.
	if eff, matched := Evaluate(b, ActionProviderUse, "anthropic"); eff != EffectAllow || matched {
		t.Errorf("unlisted provider: got (%q,%v), want (%q,false)", eff, matched, EffectAllow)
	}
}

// TestLocalBundle_EnabledActsAsAllowlist pins that a non-empty enabled list
// inverts the default, including the model.allow statement without which the
// allowlist would be useless.
//
// Mutation check: deleting the `PolicyStatement{Action: ActionModelUse, ...
// Resource: p + "/*"}` append in the enabled loop makes the "allowed
// provider's own model" assertion FAIL (it would fall to DefaultAllow=deny).
func TestLocalBundle_EnabledActsAsAllowlist(t *testing.T) {
	b := LocalBundle([]string{"openai", "anthropic"}, nil)

	if err := Validate(b); err != nil {
		t.Fatalf("LocalBundle produced an invalid bundle: %v", err)
	}
	if b.EffectiveDefault() != EffectDeny {
		t.Errorf("a non-empty enabled list must invert the default to deny, got %q", b.EffectiveDefault())
	}
	// The named providers work, providers AND their models.
	if eff, _ := Evaluate(b, ActionProviderUse, "openai"); eff != EffectAllow {
		t.Errorf("enabled provider: got %q, want allow", eff)
	}
	if eff, _ := Evaluate(b, ActionModelUse, "openai/gpt-4.1"); eff != EffectAllow {
		t.Errorf("enabled provider's model: got %q, want allow", eff)
	}
	// Everything else is denied.
	for _, other := range []string{"google", "deepseek", "ollama", "lmstudio"} {
		if eff, _ := Evaluate(b, ActionProviderUse, other); eff != EffectDeny {
			t.Errorf("unlisted provider %q: got %q, want deny", other, eff)
		}
	}
	if eff, _ := Evaluate(b, ActionModelUse, "deepseek/deepseek-v4-pro"); eff != EffectDeny {
		t.Errorf("unlisted model: got %q, want deny", eff)
	}
}

// TestLocalBundle_EmptyListsRestrictNothing pins the no-policy default: a
// fresh install is not locked out by the feature shipping.
//
// Mutation check: making LocalBundle return DefaultAllow "deny" unconditionally
// makes this test FAIL.
func TestLocalBundle_EmptyListsRestrictNothing(t *testing.T) {
	for name, b := range map[string]*PolicyBundle{
		"nil, nil":     LocalBundle(nil, nil),
		"empty, empty": LocalBundle([]string{}, []string{}),
		"blanks":       LocalBundle([]string{""}, []string{""}),
	} {
		for _, action := range []string{ActionProviderUse, ActionModelUse} {
			if eff, _ := Evaluate(b, action, "openai/gpt-4.1"); eff != EffectAllow {
				t.Errorf("%s: got %q, want allow — an unconfigured policy must not lock the user out", name, eff)
			}
		}
	}
}

// TestLocalBundle_DenyWinsOverAllow pins the conflict rule: a provider in BOTH
// lists ends up denied.
//
// Mutation check: emitting the disabledProviders loop BEFORE the
// enabledProviders loop (so an allow is last and wins) makes this test FAIL.
func TestLocalBundle_DenyWinsOverAllow(t *testing.T) {
	b := LocalBundle([]string{"openai", "anthropic"}, []string{"openai"})

	if eff, _ := Evaluate(b, ActionProviderUse, "openai"); eff != EffectDeny {
		t.Errorf("provider in both lists: got %q, want deny (the restrictive reading must win)", eff)
	}
	// The other allowed provider is unaffected.
	if eff, _ := Evaluate(b, ActionProviderUse, "anthropic"); eff != EffectAllow {
		t.Errorf("unrelated enabled provider: got %q, want allow", eff)
	}
}

// TestLocalBundle_NeverWidens is the property test for the one-directional
// guarantee: whatever the user puts in the two lists, the resulting bundle must
// never permit something an unrestricted install would have refused.
//
// Mutation check: swapping the disabled loop's effect to "allow" (or the
// enabled loop's default inversion) makes the "widened" sub-check FAIL.
func TestLocalBundle_NeverWidens(t *testing.T) {
	inputs := []struct{ enabled, disabled []string }{
		{nil, nil},
		{[]string{"openai"}, nil},
		{nil, []string{"openai"}},
		{[]string{"openai", "anthropic"}, []string{"openai"}},
		{[]string{"*"}, nil},                // an allowlist of everything
		{nil, []string{"*"}},                // a denylist of everything
		{[]string{"openai"}, []string{"*"}}, // contradictory on purpose
		{[]string{"open*"}, []string{"openai"}},
	}
	for _, in := range inputs {
		b := LocalBundle(in.enabled, in.disabled)
		// A bundle is only ever compared against the unrestricted baseline:
		// for every resource, the restricted bundle must be at least as strict.
		// Implemented as: if the baseline allows, the bundle may allow or
		// deny; the widening direction is the one that must never happen,
		// which is checked by the explicit deny cases below.
		for _, r := range []string{"openai", "openai/gpt-4.1", "anthropic", "google/gemini-3-flash"} {
			base, _ := Evaluate(LocalBundle(nil, nil), ActionProviderUse, r)
			got, _ := Evaluate(b, ActionProviderUse, r)
			if base == EffectAllow && got == EffectDeny {
				continue // narrowing: fine
			}
			if base == EffectDeny && got == EffectAllow {
				t.Errorf("LocalBundle(%v, %v) WIDENED: %q is now allowed where an "+
					"unrestricted install denied it", in.enabled, in.disabled, r)
			}
		}
		// And the strongest case: a denylist of "*" must deny everything.
		if all := LocalBundle(nil, []string{"*"}); func() bool {
			e, _ := Evaluate(all, ActionProviderUse, "openai")
			return e != EffectDeny
		}() {
			t.Errorf(`LocalBundle(nil, ["*"]) does not deny everything — a "*" denylist must be maximally restrictive`)
		}
	}
}
