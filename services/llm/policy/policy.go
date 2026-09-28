// Package policy is the open-source foundation of the LLM provider policy
// feature: the data shapes and the evaluation logic that decide which
// providers and models a user may use.
//
// Everything in this package is UNSIGNED and SELF-IMPOSED. A user sets their
// own allow/deny list in config.yaml and it constrains their own installation
// and nothing else. The enterprise downstream (a private fork) will add signed
// bundles, delivery, and enforcement; this package is the engine that fork
// overrides enforcement for. The seam pattern is services/scanner's
// scanAllowed var: OSS carries the evaluation, the fork carries enforcement.
//
// This package deliberately has NO dependency on the catalog or on services —
// it is pure data plus pure logic, so both OSS and the fork can share it
// without either side's import graph leaking into the other.
package policy

import (
	"fmt"
	"strings"
	"time"
)

// Action names the kind of thing a statement decides about. A `model.use`
// statement is distinct from a `provider.use` statement on the same resource:
// model ids are `provider/model`, so a `model.use` statement on `openai/*`
// governs model selection while a `provider.use` statement on `openai`
// governs the provider itself.
const (
	ActionProviderUse = "provider.use"
	ActionModelUse    = "model.use"
)

// Effect values. An Effect is what a matching statement decides; the bundle's
// DefaultApply is what happens when nothing matches.
const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Effect is the decision a statement (or a bundle default) carries.
type Effect string

// PolicyStatement is one allow/deny rule. Both Action and Resource are
// wildcard patterns: `*` matches any run of characters, so `*` matches
// everything and `openai/gpt-5*` matches a model prefix.
//
// Effect is a plain string in the serialized form (this shape is meant to be
// read from YAML) and is normalized through ParseEffect on evaluation, so a
// hand-edited config cannot smuggle in an unknown effect: anything that is not
// exactly "deny" is treated as allow. Because a local policy may only NARROW
// what is usable (see the package doc), an unparseable effect must never
// resolve to allow-by-accident for a statement the user wrote as a restriction
// — Validate reports it, and LocalBundle never emits an unknown effect.
type PolicyStatement struct {
	Action   string `yaml:"action" mapstructure:"action" json:"action"`
	Effect   string `yaml:"effect" mapstructure:"effect" json:"effect"`
	Resource string `yaml:"resource" mapstructure:"resource" json:"resource"`
}

// EgressPolicy is a data shape only. OSS does not enforce egress: the
// baseURL half of request-time policy is enterprise-only and out of scope for
// this package. The field exists so a signed bundle from the fork round-trips
// through the same shape.
type EgressPolicy struct {
	Mode     string   `yaml:"mode" mapstructure:"mode" json:"mode"`
	BaseURLs []string `yaml:"base_urls" mapstructure:"base_urls" json:"base_urls"`
}

// PolicyBundle is the unsigned bundle shape.
//
// There is deliberately NO signature field. OSS policy is self-imposed: the
// user is the only author and the only subject, so there is nothing to attest
// to and a signature would imply a trust relationship that does not exist
// here. The enterprise fork carries its own signed bundle type with a
// signature and a pinned issuer keyring; it converts into these statements
// before evaluation rather than this type growing a `signature` field.
//
// IssuedAt/ExpiresAt are carried for shape compatibility with the signed
// bundle. Evaluate does NOT act on ExpiresAt, and that is a deliberate
// fail-narrowing decision, not an omission: dropping a bundle's statements
// because it expired would REMOVE restrictions, which widens what a user (or
// an org) has restricted and breaks the one-directional rule this package
// guarantees. See Evaluate.
type PolicyBundle struct {
	SchemaVersion int               `yaml:"schema_version" mapstructure:"schema_version" json:"schema_version"`
	Version       string            `yaml:"version" mapstructure:"version" json:"version"`
	IssuedAt      time.Time         `yaml:"issued_at" mapstructure:"issued_at" json:"issued_at"`
	ExpiresAt     *time.Time        `yaml:"expires_at,omitempty" mapstructure:"expires_at" json:"expires_at,omitempty"`
	Org           string            `yaml:"org" mapstructure:"org" json:"org"`
	DefaultAllow  string            `yaml:"default_allow" mapstructure:"default_allow" json:"default_allow"`
	Statements    []PolicyStatement `yaml:"statements" mapstructure:"statements" json:"statements"`
	Egress        *EgressPolicy     `yaml:"egress,omitempty" mapstructure:"egress" json:"egress,omitempty"`
	CatalogSource string            `yaml:"catalog_source,omitempty" mapstructure:"catalog_source" json:"catalog_source,omitempty"`
}

// CurrentSchemaVersion is the SchemaVersion this build understands.
const CurrentSchemaVersion = 1

// ParseEffect normalizes a serialized effect string. Only "deny" is deny;
// everything else is allow, because an unrecognized effect must not silently
// become a restriction the user cannot see. Callers that care use Validate.
func ParseEffect(s string) Effect {
	if strings.EqualFold(strings.TrimSpace(s), string(EffectDeny)) {
		return EffectDeny
	}
	return EffectAllow
}

// EffectiveDefault returns the bundle's fallback effect for a request that
// matches no statement. An unset or unrecognized DefaultAllow is allow: this
// is a self-imposed local policy, and a fresh install with no policy configured
// must not come up locked. Narrowing is expressed with statements, and a
// bundle that wants deny-by-default says so explicitly.
func (b *PolicyBundle) EffectiveDefault() Effect {
	if b == nil {
		return EffectAllow
	}
	return ParseEffect(b.DefaultAllow)
}

// Evaluate decides one (action, resource) pair against the bundle and reports
// whether any statement matched.
//
// Resolution is FIND-LAST-WINS: statements are scanned in order and the last
// statement matching both Action and Resource decides. That is what makes a
// local bundle able to express "allow everything except X" as
// [allow *, deny X] and what lets LocalBundle put explicit denies last so a
// deny beats an earlier allow for the same resource.
//
// Both Action and Resource are matched as wildcard patterns. A nil bundle (or
// a nil-safe empty one) allows everything: the request-time caller treats
// "no policy configured" as "no restriction", matching EffectiveDefault.
//
// matched is false when no statement applied and the returned effect is the
// bundle default. Callers that must distinguish "nothing said" from "said
// allow" (a widening decision) can, and callers that only need the decision
// can ignore it.
func Evaluate(b *PolicyBundle, action, resource string) (Effect, bool) {
	if b == nil {
		return EffectAllow, false
	}
	var decided Effect
	matched := false
	for _, st := range b.Statements {
		if !MatchWildcard(st.Action, action) || !MatchWildcard(st.Resource, resource) {
			continue
		}
		decided = ParseEffect(st.Effect)
		matched = true // later matches overwrite: find-last-wins
	}
	if !matched {
		return b.EffectiveDefault(), false
	}
	return decided, true
}

// MatchWildcard reports whether s matches pattern, where `*` in pattern
// matches any run of characters (including none) and every other character
// must match literally. Matching is case-SENSITIVE: catalog ids are lowercase
// and a case-insensitive match would silently widen a narrowly written
// restriction.
//
// An EMPTY pattern never matches a non-empty string, so a statement that lost
// its resource in a hand edit cannot come to mean "match everything". The one
// exception is the empty string matching the empty string, which is harmless
// and never reached for a real resource (callers skip the model check when no
// model is configured).
func MatchWildcard(pattern, s string) bool {
	// Iterative glob match with a single backtrack point: O(n*m) worst case,
	// no recursion, no regexp compilation per call. This runs on every
	// outbound request, so it is written to allocate nothing.
	pi, si := 0, 0
	star, mark := -1, 0
	for si < len(s) {
		switch {
		case pi < len(pattern) && pattern[pi] == s[si]:
			pi++
			si++
		case pi < len(pattern) && pattern[pi] == '*':
			star = pi
			mark = si
			pi++
		case star >= 0:
			// Backtrack: let the last `*` absorb one more character.
			pi = star + 1
			mark++
			si = mark
		default:
			return false
		}
	}
	for pi < len(pattern) && pattern[pi] == '*' {
		pi++
	}
	return pi == len(pattern)
}

// Validate reports whether the bundle is internally well-formed: a supported
// schema version, a recognized default, and statements with a known action and
// effect. It is used on a locally-authored bundle so a typo surfaces as an
// error rather than as a rule that quietly never matches.
func Validate(b *PolicyBundle) error {
	if b == nil {
		return fmt.Errorf("policy: nil bundle")
	}
	if b.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("policy: unsupported schema_version %d (want %d)", b.SchemaVersion, CurrentSchemaVersion)
	}
	if d := strings.ToLower(strings.TrimSpace(b.DefaultAllow)); d != "" &&
		d != string(EffectAllow) && d != string(EffectDeny) {
		return fmt.Errorf("policy: default_allow %q must be %q or %q", b.DefaultAllow, EffectAllow, EffectDeny)
	}
	for i, st := range b.Statements {
		switch st.Action {
		case ActionProviderUse, ActionModelUse:
		default:
			return fmt.Errorf("policy: statement %d has action %q, want %q or %q",
				i, st.Action, ActionProviderUse, ActionModelUse)
		}
		if e := strings.ToLower(strings.TrimSpace(st.Effect)); e != string(EffectAllow) && e != string(EffectDeny) {
			return fmt.Errorf("policy: statement %d has effect %q, want %q or %q",
				i, st.Effect, EffectAllow, EffectDeny)
		}
	}
	return nil
}

// ResourceID builds the canonical `provider/model` resource for a model.use
// check. Returning the bare provider id when model is empty keeps a
// model.use check from accidentally evaluating the provider as a model.
func ResourceID(provider, model string) string {
	if model == "" {
		return provider
	}
	return provider + "/" + model
}
