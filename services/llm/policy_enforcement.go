package llm

import (
	"fmt"

	"pairadmin/services/config"
	"pairadmin/services/llm/catalog"
	"pairadmin/services/llm/policy"
)

// This file holds the two OSS enforcement points for LLM policy and the single
// seam the enterprise fork overrides.
//
// The seam is deliberately shaped like services/scanner's scanAllowed: a
// package-level var holding the OSS behavior, which a fork replaces wholesale
// to add signed-bundle delivery and enforcement. OSS therefore carries the
// evaluation and the fork carries enforcement, and this repo never grows any
// knowledge of signing, issuers, or keyrings.
//
// Both enforcement points are ONE-DIRECTIONAL: they can only remove providers
// and models from what is usable, never add any. A policy that somehow
// evaluated to "allow" for everything would be a no-op, not a widening, so
// there is no input to this file that can make a restricted install less
// restricted.

// enforcePolicy is the deny-at-catalog-finalize choke point. When non-nil it
// filters a provider list before the UI, activation, or any other consumer
// ever sees it.
//
// A fork overrides this var to enforce a signed bundle. Do NOT add
// enterprise behavior to the default below — the default is the OSS
// self-restriction and nothing else, exactly as scanAllowed is.
var enforcePolicy = filterCatalogByPolicy

// filterCatalogByPolicy is the OSS default: drop every provider denied by
// `provider.use`, then drop every model denied by `model.use`. It returns a
// new slice and never mutates the caller's, and it never adds a provider or
// model that was not in the input.
func filterCatalogByPolicy(b *policy.PolicyBundle, providers []catalog.Provider) []catalog.Provider {
	out := make([]catalog.Provider, 0, len(providers))
	for _, p := range providers {
		if eff, _ := policy.Evaluate(b, policy.ActionProviderUse, p.ID); eff == policy.EffectDeny {
			continue
		}
		filtered := p
		if len(p.Models) > 0 {
			models := make([]catalog.Model, 0, len(p.Models))
			for _, m := range p.Models {
				if eff, _ := policy.Evaluate(b, policy.ActionModelUse,
					policy.ResourceID(p.ID, m.ID)); eff == policy.EffectDeny {
					continue
				}
				models = append(models, m)
			}
			// A provider whose every model is denied is left in place with no
			// models rather than removed: removing it would also revoke the
			// provider itself, which is a `provider.use` decision this
			// function must not make on the model's behalf.
			filtered.Models = models
		}
		out = append(out, filtered)
	}
	return out
}

// FilterCatalog applies the effective policy to a provider list. This is
// enforcement point 1: the catalog is filtered before the UI or activation
// logic sees it, so a denied provider is never even offered.
func FilterCatalog(providers []catalog.Provider) []catalog.Provider {
	if enforcePolicy == nil {
		return providers
	}
	return enforcePolicy(EffectiveBundle(), providers)
}

// EffectiveBundle builds the policy in force for this process from the user's
// config.yaml enabled_providers / disabled_providers lists.
//
// It re-reads the config on every call rather than caching, so a hand-edited
// config.yaml takes effect on the very next request instead of after a
// restart — which is also what makes the request-time re-check meaningful. The
// file is a few KB of local YAML, so the read is not the bottleneck the policy
// decision is.
//
// A config that cannot be read yields an ALLOW policy, not a deny: a corrupt
// or missing config must not lock a user out of their own LLM providers, and
// this is a self-imposed feature with no signed bundle to fail closed on. The
// enterprise fork's signed bundle is where fail-closed belongs.
func EffectiveBundle() *policy.PolicyBundle {
	cfg, err := config.LoadAppConfig()
	if err != nil || cfg == nil {
		return policy.LocalBundle(nil, nil)
	}
	return policy.LocalBundle(cfg.EnabledProviders, cfg.DisabledProviders)
}

// CheckRequest is enforcement point 2: the request-time re-check, run on every
// outbound call rather than trusting the catalog filter. The catalog is read
// once per settings view, so a hand-edited config, a stale cached list, or a
// model typed straight into the UI would otherwise bypass point 1 entirely.
//
// It checks `provider.use` on the provider id and, when a model is configured,
// `model.use` on the canonical provider/model id. A provider with no model
// configured skips the model check rather than evaluating a bare provider id
// as a model, which would let a provider deny its own models by accident.
func CheckRequest(provider, model string) error {
	return checkRequestAgainst(EffectiveBundle(), provider, model)
}

// checkRequestAgainst is the pure decision half of CheckRequest, split out so
// the model.use branch is testable with an injected bundle: the OSS config
// lists are provider-granular, so a model-level deny is not reachable through
// config.yaml alone (it comes from the fork's signed bundle, or from a future
// OSS config field). Keeping the decision pure means that branch is covered
// without inventing a config knob that does not exist.
func checkRequestAgainst(b *policy.PolicyBundle, provider, model string) error {
	if provider == "" || provider == "disabled" {
		return nil // the existing opt-out semantics, not a policy decision
	}
	if eff, _ := policy.Evaluate(b, policy.ActionProviderUse, provider); eff == policy.EffectDeny {
		return fmt.Errorf("provider %q is not permitted by the local LLM policy "+
			"(disabled_providers in config.yaml); remove it to re-enable", provider)
	}
	if model != "" {
		if eff, _ := policy.Evaluate(b, policy.ActionModelUse,
			policy.ResourceID(provider, model)); eff == policy.EffectDeny {
			return fmt.Errorf("model %q is not permitted by the local LLM policy "+
				"(enabled_providers/disabled_providers in config.yaml)", policy.ResourceID(provider, model))
		}
	}
	return nil
}
