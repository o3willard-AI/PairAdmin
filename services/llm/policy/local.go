package policy

import "strings"

// LocalBundle builds the implicit bundle behind config.yaml's
// enabled_providers / disabled_providers lists. This is the entire OSS policy
// feature: a user restricting their own installation, with no enterprise fork
// present and no signature anywhere.
//
// It goes through the SAME Evaluate engine as any other bundle — there is no
// second, simpler code path for the local lists, which is what makes the
// downstream fork's override meaningful rather than a parallel implementation.
//
// The bundle is guaranteed NARROWING-ONLY: every entry produces either a deny
// statement or an allow statement that is bounded by DefaultAllow=deny. There
// is no input to this function that can produce a bundle permitting more than
// an unrestricted install. That matters because this function's output is
// attacker-influenceable (anyone who can write config.yaml can hand-edit it),
// and a "restriction" that could widen would be a privilege-escalation
// primitive dressed as a security feature.
//
// How the two lists combine, given find-last-wins:
//
//   - enabled non-empty  -> DefaultAllow "deny", one allow per enabled
//     provider, plus an allow for that provider's models. This is an
//     allowlist: everything unlisted stays denied.
//   - enabled empty      -> DefaultAllow "allow", so the deny statements
//     below are what restrict anything.
//   - disabled non-empty -> one deny per disabled provider, plus a deny for its
//     models.
//   - both non-empty     -> the denies are emitted AFTER the allows, so a
//     provider that is both enabled and disabled ends up denied. Deny wins on
//     conflict: the restrictive reading is the safe one.
func LocalBundle(enabledProviders, disabledProviders []string) *PolicyBundle {
	// Normalize BEFORE deciding anything about the default. Deciding on the
	// raw slice length while skipping blank entries inside the loop is a
	// lockout: `enabled_providers: [""]` (a stray blank from a hand-edited or
	// machine-written config) would invert the default to deny and then emit
	// no allow statement at all, denying every provider with nothing on
	// screen to explain why.
	enabled := cleanEntries(enabledProviders)
	disabled := cleanEntries(disabledProviders)

	b := &PolicyBundle{
		SchemaVersion: CurrentSchemaVersion,
		Version:       "local",
		Org:           "local",
		DefaultAllow:  string(EffectAllow),
	}

	if len(enabled) > 0 {
		// Allowlist mode: invert the default so "not listed" is denied.
		b.DefaultAllow = string(EffectDeny)
		for _, p := range enabled {
			b.Statements = append(b.Statements,
				PolicyStatement{Action: ActionProviderUse, Effect: string(EffectAllow), Resource: p},
				// Without this, DefaultAllow=deny would also deny every
				// model of an allowed provider, making the allowlist
				// useless for the picker.
				PolicyStatement{Action: ActionModelUse, Effect: string(EffectAllow), Resource: p + "/*"},
			)
		}
	}

	// Denies last: find-last-wins makes them override the allows above.
	for _, p := range disabled {
		b.Statements = append(b.Statements,
			PolicyStatement{Action: ActionProviderUse, Effect: string(EffectDeny), Resource: p},
			PolicyStatement{Action: ActionModelUse, Effect: string(EffectDeny), Resource: p + "/*"},
		)
	}
	return b
}

// cleanEntries trims surrounding whitespace and drops blank entries. This is
// normalization, not widening: a blank can never match a real provider id, and
// whitespace around a real id is a typo, not an intent to match a padded
// string.
func cleanEntries(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
