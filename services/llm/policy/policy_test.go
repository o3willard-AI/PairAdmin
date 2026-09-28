package policy

import (
	"testing"
	"time"
)

func stmt(action, effect, resource string) PolicyStatement {
	return PolicyStatement{Action: action, Effect: effect, Resource: resource}
}

// TestEvaluate_FindLastWins pins the core resolution rule: the LAST statement
// matching both action and resource decides, not the first.
//
// Mutation check: changing `decided = ParseEffect(st.Effect)` in Evaluate's
// loop to `if !matched { decided = ... }` — i.e. first-wins — makes this test
// FAIL on TestEvaluate_FindLastWins/overrides. The first statement in the
// bundle denies, the later one allows; first-wins would deny and the
// sub-assertion for "allow" would not hold.
func TestEvaluate_FindLastWins(t *testing.T) {
	b := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements: []PolicyStatement{
			stmt(ActionProviderUse, "deny", "openai"),
			stmt(ActionProviderUse, "allow", "openai"), // later wins
		},
	}
	if eff, matched := Evaluate(b, ActionProviderUse, "openai"); eff != EffectAllow || !matched {
		t.Errorf("find-last-wins: got (%q, %v), want (%q, true)", eff, matched, EffectAllow)
	}

	// And the reverse order really does deny — the test is not passing
	// because every statement evaluates to the same thing.
	b2 := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements: []PolicyStatement{
			stmt(ActionProviderUse, "allow", "openai"),
			stmt(ActionProviderUse, "deny", "openai"), // later wins
		},
	}
	if eff, _ := Evaluate(b2, ActionProviderUse, "openai"); eff != EffectDeny {
		t.Errorf("find-last-wins reversed: got %q, want %q", eff, EffectDeny)
	}
}

// TestEvaluate_DefaultAllowFallback pins that a request matching no statement
// falls through to the bundle default, and reports matched=false so a caller
// can tell "nothing said" from "said allow".
//
// Mutation check: returning `EffectAllow, false` unconditionally instead of
// `b.EffectiveDefault(), false` makes the deny-default sub-case FAIL.
func TestEvaluate_DefaultAllowFallback(t *testing.T) {
	for _, tc := range []struct {
		name    string
		def     string
		want    Effect
		matched bool
	}{
		{"default allow", "allow", EffectAllow, false},
		{"default deny", "deny", EffectDeny, false},
		{"unset default is allow", "", EffectAllow, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &PolicyBundle{
				SchemaVersion: CurrentSchemaVersion,
				DefaultAllow:  tc.def,
				Statements:    []PolicyStatement{stmt(ActionProviderUse, "deny", "openai")},
			}
			eff, matched := Evaluate(b, ActionProviderUse, "anthropic")
			if eff != tc.want {
				t.Errorf("got effect %q, want %q", eff, tc.want)
			}
			if matched != tc.matched {
				t.Errorf("got matched %v, want %v (matched must be false when no statement applied)", matched, tc.matched)
			}
		})
	}
}

// TestEvaluate_WildcardOnActionAndResource pins wildcard matching on BOTH
// fields independently: a statement's Action and Resource are each a pattern.
//
// Mutation check: replacing MatchWildcard(st.Action, action) with
// `st.Action == action` (exact action match) makes the `*` action sub-cases
// FAIL; replacing the Resource comparison likewise fails the model-prefix case.
func TestEvaluate_WildcardOnActionAndResource(t *testing.T) {
	b := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements: []PolicyStatement{
			stmt("*", "deny", "*"), // matches everything
		},
	}
	for _, tc := range []struct{ action, resource string }{
		{ActionProviderUse, "openai"},
		{ActionModelUse, "openai/gpt-5.6-luna"},
		{"some.other/action", "whatever"},
	} {
		if eff, matched := Evaluate(b, tc.action, tc.resource); eff != EffectDeny || !matched {
			t.Errorf("(*,*) did not match %q/%q: got (%q,%v)", tc.action, tc.resource, eff, matched)
		}
	}

	// Model prefix pattern.
	prefix := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements:    []PolicyStatement{stmt(ActionModelUse, "deny", "openai/gpt-5*")},
	}
	if eff, _ := Evaluate(prefix, ActionModelUse, "openai/gpt-5.6-luna"); eff != EffectDeny {
		t.Errorf("model prefix: got %q, want deny", eff)
	}
	// ...and a non-matching prefix stays at the default.
	if eff, matched := Evaluate(prefix, ActionModelUse, "openai/gpt-4.1"); eff != EffectAllow || matched {
		t.Errorf("non-matching prefix: got (%q,%v), want (%q,false)", eff, matched, EffectAllow)
	}
	// ...and a prefix on a DIFFERENT provider must not match: resources are
	// provider-qualified, so "gpt-5*" must not leak across providers.
	if eff, _ := Evaluate(prefix, ActionModelUse, "anthropic/gpt-5.6-luna"); eff != EffectAllow {
		t.Errorf("prefix matched across providers: got %q, want allow", eff)
	}
}

// TestEvaluate_ProviderUseAndModelUseAreDistinct pins that action and resource
// are evaluated independently: a `model.use` statement does not decide a
// `provider.use` question even on the same resource string.
//
// Mutation check: dropping the `st.Action == action` half of the match (so
// only Resource is compared) makes the provider-use sub-cases FAIL — a
// model.use deny on "openai" would then wrongly deny the provider itself.
func TestEvaluate_ProviderUseAndModelUseAreDistinct(t *testing.T) {
	b := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements: []PolicyStatement{
			stmt(ActionModelUse, "deny", "openai/gpt-4o-mini"),
		},
	}

	// A model.use deny says nothing about the provider.
	if eff, matched := Evaluate(b, ActionProviderUse, "openai"); eff != EffectAllow || matched {
		t.Errorf("model.use leaked into provider.use: got (%q,%v), want (%q,false)",
			eff, matched, EffectAllow)
	}
	// It does deny that model.
	if eff, _ := Evaluate(b, ActionModelUse, "openai/gpt-4o-mini"); eff != EffectDeny {
		t.Errorf("model.use did not deny its own model: got %q", eff)
	}
	// A provider.use statement does not decide a model.use question either.
	pb := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		Statements:    []PolicyStatement{stmt(ActionProviderUse, "deny", "openai")},
	}
	if eff, _ := Evaluate(pb, ActionModelUse, "openai/gpt-4o-mini"); eff != EffectAllow {
		t.Errorf("provider.use leaked into model.use: got %q, want allow", eff)
	}
}

// TestEvaluate_NilBundleAllows pins that a nil bundle is allow + not-matched.
// Without this, a caller that failed to build a bundle would deny everything,
// turning any construction bug into a total lockout.
//
// Mutation check: changing `if b == nil { return EffectAllow, false }` to
// `return EffectDeny, false` makes this test FAIL.
func TestEvaluate_NilBundleAllows(t *testing.T) {
	eff, matched := Evaluate(nil, ActionProviderUse, "openai")
	if eff != EffectAllow {
		t.Errorf("nil bundle: got %q, want %q", eff, EffectAllow)
	}
	if matched {
		t.Error("nil bundle must not report a match")
	}
}

// TestEvaluate_ExpiredBundleStillAppliesStatements pins the deliberate
// fail-NARROWING decision: Evaluate ignores ExpiresAt.
//
// The tempting alternative is to drop a bundle's statements once it expires.
// That would REMOVE restrictions, widening what a user has limited, and
// breaking the one-directional rule this package guarantees. So expiry is a
// signed-bundle concept the enterprise fork owns; here an expired local
// bundle keeps applying.
//
// Mutation check: adding an expiry short-circuit to Evaluate (returning the
// default when now > *b.ExpiresAt) makes this test FAIL — the deny would stop
// applying and the provider would become allowed again.
func TestEvaluate_ExpiredBundleStillAppliesStatements(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	b := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "allow",
		ExpiresAt:     &past,
		IssuedAt:      past.Add(-time.Hour),
		Statements:    []PolicyStatement{stmt(ActionProviderUse, "deny", "openai")},
	}
	if eff, _ := Evaluate(b, ActionProviderUse, "openai"); eff != EffectDeny {
		t.Errorf("expired bundle stopped denying (got %q): expiry must not widen, only "+
			"an enterprise signed bundle may treat expiry that way", eff)
	}
}

// TestMatchWildcard pins the glob itself, including the two safety properties
// the policy depends on: case sensitivity, and an empty pattern not matching
// everything.
//
// Mutation check: making MatchWildcard case-insensitive (strings.ToLower on
// both operands) makes the OpenAI sub-case FAIL; making an empty pattern
// match everything makes the empty-pattern sub-case FAIL.
func TestMatchWildcard(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "", true},
		{"*", "anything/at/all", true},
		{"openai", "openai", true},
		{"openai", "openaix", false},
		{"openai", "open", false},
		{"openai/*", "openai/gpt-5.6-luna", true},
		{"openai/*", "openai/", true},
		{"openai/*", "anthropic/gpt-5.6", false},
		{"openai/gpt-5*", "openai/gpt-5.6-luna", true},
		{"openai/gpt-5*", "openai/gpt-4.1", false},
		{"*gpt-5*", "openai/gpt-5.6-luna", true},
		{"*gpt-5*", "openai/gpt-4.1", false},
		{"a*b*c", "axxbyyc", true},
		{"a*b*c", "axxbyy", false},
		// Case sensitivity: a lowercased resource must not match a
		// differently-cased pattern, or a typo would widen a restriction.
		{"openai", "OpenAI", false},
		// An empty pattern is not a match-everything.
		{"", "openai", false},
	}
	for _, tc := range cases {
		if got := MatchWildcard(tc.pattern, tc.s); got != tc.want {
			t.Errorf("MatchWildcard(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

// TestValidate_RejectsMalformedBundles pins that a hand-edited bundle with a
// bad schema version, default, action, or effect is reported rather than
// silently becoming a rule that never matches.
//
// Mutation check: returning nil at the top of Validate (early `return nil`)
// makes every sub-case FAIL.
func TestValidate_RejectsMalformedBundles(t *testing.T) {
	ok := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		DefaultAllow:  "deny",
		Statements:    []PolicyStatement{stmt(ActionProviderUse, "deny", "openai")},
	}
	if err := Validate(ok); err != nil {
		t.Fatalf("valid bundle rejected: %v", err)
	}

	bad := []*PolicyBundle{
		{SchemaVersion: 99, DefaultAllow: "allow"},
		{SchemaVersion: CurrentSchemaVersion, DefaultAllow: "maybe"},
		{SchemaVersion: CurrentSchemaVersion, DefaultAllow: "allow",
			Statements: []PolicyStatement{stmt("bogus.use", "deny", "openai")}},
		{SchemaVersion: CurrentSchemaVersion, DefaultAllow: "allow",
			Statements: []PolicyStatement{stmt(ActionProviderUse, "perhaps", "openai")}},
	}
	for i, b := range bad {
		if err := Validate(b); err == nil {
			t.Errorf("malformed bundle %d was accepted: %+v", i, b)
		}
	}
	if err := Validate(nil); err == nil {
		t.Error("nil bundle accepted by Validate")
	}
}

// TestResourceID pins the canonical provider/model resource, and the
// model-less case that keeps a bare provider id out of a model.use check.
func TestResourceID(t *testing.T) {
	if got := ResourceID("openai", "gpt-4.1"); got != "openai/gpt-4.1" {
		t.Errorf("got %q, want %q", got, "openai/gpt-4.1")
	}
	if got := ResourceID("openai", ""); got != "openai" {
		t.Errorf("model-less: got %q, want %q", got, "openai")
	}
}
