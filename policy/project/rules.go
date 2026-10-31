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

// projectRules turns each package rule into a match.Rule. It drops a trust
// entry with an unknown field, and each rule that match.Normalize drops. A
// block entry with an unknown field stays, and the field is reported.
func (p *Projection) projectRules(rules []*corev1.PackageRule) {
	for _, pr := range rules {
		r := toMatchRule(pr)
		unknown := messageUnknown(pr.ProtoReflect(), pathPackageRules)

		if r.Effect == match.EffectNone {
			p.dropRule(r.ID, fmt.Sprintf("%v: %d", match.ErrUnknownEffect, int32(pr.GetEffect())))
			continue
		}

		if r.Effect == match.EffectTrust && len(unknown) > 0 {
			p.dropRule(r.ID, fmt.Sprintf("trust entry has an unknown field: %s", unknown[0].Path))
			continue
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

		p.Rules = append(p.Rules, normalized)
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
