package project

import (
	"errors"
	"fmt"

	corev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/core/v1"

	"github.com/safedep/dry/policy/match"
)

// scopes maps each trust scope that the tool knows. A scope that is not in
// the map is ignored, because the scope is an open enum.
var scopes = map[corev1.TrustScope]match.Scope{
	corev1.TrustScope_TRUST_SCOPE_MALWARE:             match.ScopeMalware,
	corev1.TrustScope_TRUST_SCOPE_COOLDOWN:            match.ScopeCooldown,
	corev1.TrustScope_TRUST_SCOPE_VULNERABILITY:       match.ScopeVulnerability,
	corev1.TrustScope_TRUST_SCOPE_LICENSE:             match.ScopeLicense,
	corev1.TrustScope_TRUST_SCOPE_NAMED_VULNERABILITY: match.ScopeNamedVulnerability,
}

// justifications maps each justification that the tool knows. The contract
// does not check the value of a resolved rule, so a later value must not
// drop the entry. It becomes match.JustificationNone, and the tool shows the
// reason of the rule.
var justifications = map[corev1.VulnerabilityJustification]match.Justification{
	corev1.VulnerabilityJustification_VULNERABILITY_JUSTIFICATION_COMPONENT_NOT_PRESENT:                             match.JustificationComponentNotPresent,
	corev1.VulnerabilityJustification_VULNERABILITY_JUSTIFICATION_VULNERABLE_CODE_NOT_PRESENT:                       match.JustificationVulnerableCodeNotPresent,
	corev1.VulnerabilityJustification_VULNERABILITY_JUSTIFICATION_VULNERABLE_CODE_NOT_IN_EXECUTE_PATH:               match.JustificationVulnerableCodeNotInExecutePath,
	corev1.VulnerabilityJustification_VULNERABILITY_JUSTIFICATION_VULNERABLE_CODE_CANNOT_BE_CONTROLLED_BY_ADVERSARY: match.JustificationVulnerableCodeCannotBeControlledByAdversary,
	corev1.VulnerabilityJustification_VULNERABILITY_JUSTIFICATION_INLINE_MITIGATIONS_ALREADY_EXIST:                  match.JustificationInlineMitigationsAlreadyExist,
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
		// match.Normalize below repeats ErrNoKnownScope. It runs after the
		// checks that decide the drop reason of a trust entry first.
		r, err := ToMatchRule(pr)
		if errors.Is(err, match.ErrUnknownEffect) {
			p.dropRule(r.ID, err.Error())
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

// ToMatchRule converts a package rule of the contract to a match.Rule. It
// is the one copy of the mapping, so the rule that a server checks is the
// rule that a tool applies. It has two callers:
//   - the projection of Decode, which then drops or keeps the rule by the
//     projection rules;
//   - a server that checks a rule before it stores it, with match.Validate.
//
// A scope that the package does not know is ignored, because the scope is an
// open enum. A justification that it does not know becomes
// match.JustificationNone. The rule keeps its id, also when it has none.
//
// It returns the converted rule with an error that wraps:
//   - match.ErrUnknownEffect, for an effect that is not trust or block;
//   - match.ErrNoKnownScope, for a trust entry with no scope that the
//     package knows.
//
// It does not apply the other checks of match.Normalize, such as the id and
// the expiry. match.Validate applies them.
func ToMatchRule(pr *corev1.PackageRule) (match.Rule, error) {
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

	for _, v := range pr.GetVulnerabilities() {
		r.Vulnerabilities = append(r.Vulnerabilities, match.WaivedVulnerability{
			ID:            v.GetId(),
			Justification: justifications[v.GetJustification()],
		})
	}

	if pr.HasExpiresAt() {
		r.ExpiresAt = pr.GetExpiresAt().AsTime()
	}

	switch {
	case r.Effect == match.EffectNone:
		return r, fmt.Errorf("%w: %d", match.ErrUnknownEffect, int32(pr.GetEffect()))
	case r.Effect == match.EffectTrust && len(r.Scopes) == 0:
		return r, match.ErrNoKnownScope
	default:
		return r, nil
	}
}
