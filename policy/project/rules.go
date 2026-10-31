package project

import (
	"fmt"

	corev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/core/v1"

	"github.com/safedep/dry/policy/match"
)

// scopes maps each trust scope that the tool knows. A scope that is not in
// the map is ignored, because the scope is an open enum.
var scopes = map[corev1.TrustScope]match.Scope{
	corev1.TrustScope_TRUST_SCOPE_MALWARE:       match.ScopeMalware,
	corev1.TrustScope_TRUST_SCOPE_COOLDOWN:      match.ScopeCooldown,
	corev1.TrustScope_TRUST_SCOPE_VULNERABILITY: match.ScopeVulnerability,
}

// projectRules turns each package rule into a match.Rule.
//   - A trust entry is dropped when it has an unknown field, has no expiry,
//     fails a contract limit of PackageRule, or fails match.Normalize.
//   - A block entry is dropped only when match.Normalize drops it. An unknown
//     field or a failed contract limit is reported, and the entry stays: a
//     block that a tool drops is a block that does not apply. NewIndex drops
//     it later when its selector does not parse.
//
// The list has no cap here. The receive limit of the fetch, 4 MiB, bounds it.
func (p *Projection) projectRules(rules []*corev1.PackageRule) {
	for _, pr := range rules {
		r := toMatchRule(pr)

		if r.Effect == match.EffectNone {
			p.dropRule(r.ID, fmt.Sprintf("%v: %d", match.ErrUnknownEffect, int32(pr.GetEffect())))
			continue
		}

		unknown := messageUnknown(pr.ProtoReflect(), pathPackageRules)
		limits, err := violations(pr)
		if err != nil {
			limits = []violation{{detail: fmt.Sprintf("cannot check the contract limits: %v", err)}}
		}

		if r.Effect == match.EffectTrust {
			if reason := trustDropReason(pr, unknown, limits); reason != "" {
				p.dropRule(r.ID, reason)
				continue
			}
		}

		normalized, err := match.Normalize(r)
		if err != nil {
			p.dropRule(r.ID, err.Error())
			continue
		}

		for _, e := range unknown {
			e.RuleID = normalized.ID
			p.Exceptions = append(p.Exceptions, e)
		}

		for _, v := range limits {
			p.Exceptions = append(p.Exceptions, Exception{
				Path:   join(pathPackageRules, v.path),
				Kind:   ExceptionInvalidValue,
				RuleID: normalized.ID,
				Detail: v.detail,
			})
		}

		p.Rules = append(p.Rules, normalized)
	}
}

// trustDropReason returns why a trust entry is dropped before
// match.Normalize, or "".
func trustDropReason(pr *corev1.PackageRule, unknown []Exception, limits []violation) string {
	switch {
	case len(unknown) > 0:
		return fmt.Sprintf("trust entry has an unknown field: %s", unknown[0].Path)
	case !pr.HasExpiresAt():
		return match.ErrNoExpiry.Error()
	case len(limits) > 0:
		return fmt.Sprintf("trust entry fails a contract limit: %s", limits[0].detail)
	default:
		return ""
	}
}

func (p *Projection) dropRule(ruleID, detail string) {
	p.Exceptions = append(p.Exceptions, Exception{
		Path:   pathPackageRules,
		Kind:   ExceptionDroppedRule,
		RuleID: ruleID,
		Detail: detail,
	})
}

func toMatchRule(pr *corev1.PackageRule) match.Rule {
	r := match.Rule{
		ID:     pr.GetRuleId(),
		PURL:   pr.GetSelector().GetPurl(),
		Vers:   pr.GetSelector().GetVers(),
		Reason: pr.GetReason(),
		Origin: match.Origin{Kind: match.OriginCloud},
	}

	switch pr.GetEffect() {
	case corev1.Effect_EFFECT_TRUST:
		r.Effect = match.EffectTrust
	case corev1.Effect_EFFECT_BLOCK:
		r.Effect = match.EffectBlock
	default:
		r.Effect = match.EffectNone
	}

	for _, s := range pr.GetScopes() {
		if scope, ok := scopes[s]; ok {
			r.Scopes = append(r.Scopes, scope)
		}
	}

	if pr.HasExpiresAt() {
		r.ExpiresAt = pr.GetExpiresAt().AsTime()
	}

	return r
}
