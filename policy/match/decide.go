package match

import (
	"slices"
	"strings"
	"time"
)

// Verdict is the verdict of the control that a tool asks about.
type Verdict int

const (
	// VerdictNone means that the control has no verdict that a rule changes,
	// such as a clean package or a version inside the cooldown window.
	VerdictNone Verdict = iota

	// VerdictMalware is a malware verdict that is not verified, such as a
	// suspicious one.
	VerdictMalware

	// VerdictVerifiedMalware is a verdict of verified malware. No trust entry
	// waives it.
	VerdictVerifiedMalware
)

// Query asks which rule applies to a package.
type Query struct {
	Package Package

	// Control is the control that asks, such as ScopeMalware or
	// ScopeCooldown. A trust entry applies only when its scopes hold the
	// control.
	Control Scope

	Verdict Verdict

	// VulnerabilityIDs are the id of the vulnerability finding and its
	// aliases, as "CVE-2024-3094" and "GHSA-jfh8-c2jp-5v3q", for the control
	// ScopeVulnerability. A trust entry with ScopeNamedVulnerability applies
	// only when one of them equals one of its ids, with no regard to case. A
	// query with no ids never matches such an entry.
	VulnerabilityIDs []string

	// Now is the time of the check. A rule applies while Now is before its
	// expiry. The zero value means time.Now().
	Now time.Time
}

// Decision is the result of a check.
type Decision struct {
	// Effect is EffectBlock, EffectTrust, or EffectNone when no rule
	// decides.
	Effect Effect

	// Rule is the rule that decided. It is the zero Rule when Effect is
	// EffectNone.
	Rule Rule

	// Vulnerability is the item of Rule.Vulnerabilities that the query
	// names, with its justification, when a trust entry with
	// ScopeNamedVulnerability decided. A tool shows it with the waived
	// finding. It is the zero value otherwise.
	Vulnerability WaivedVulnerability
}

// Decide finds the rule that applies to the package of the query:
//  1. A block entry that matches the package: EffectBlock.
//  2. A verdict of verified malware: EffectNone. No trust entry applies, for
//     any scope and any control. This is the verified malware guard.
//  3. A trust entry that matches the package and whose scopes hold the
//     control: EffectTrust. Under the control ScopeVulnerability, a trust
//     entry with ScopeNamedVulnerability also matches when the query names
//     one of its vulnerabilities.
//  4. Otherwise: EffectNone.
//
// A rule matches while the time of the query is before its expiry. When two
// rules of one effect match, the rule with the lower id decides.
//
// The version checks fail closed:
//   - A package with no version matches a block entry with a version or a
//     range, and never a trust entry with one.
//   - A version that the identity cannot order matches a block entry with a
//     range, and never a trust entry with one.
//   - A block entry with one version matches the same version text, and a
//     version that the identity orders equal to it, such as 1.0.0+build for
//     1.0.0 under npm. A trust entry with one version matches only the same
//     text.
func (ix *Index) Decide(q Query) Decision {
	if ix == nil || q.Package == nil {
		return Decision{}
	}

	now := q.Now
	if now.IsZero() {
		now = time.Now()
	}

	var block, trust *entry
	consider := func(list []*entry) {
		for _, e := range list {
			block, trust = better(e, q, now, block, trust)
		}
	}

	key := q.Package.NameKey()
	consider(ix.exact[key])
	for _, n := range ix.prefixLengths {
		if n > len(key) {
			break
		}
		consider(ix.prefixes[key[:n]])
	}

	switch {
	case block != nil:
		return Decision{Effect: EffectBlock, Rule: block.rule.clone()}
	case q.Verdict == VerdictVerifiedMalware:
		return Decision{}
	case trust != nil:
		waived, _ := namedVulnerability(&trust.rule, q)
		return Decision{Effect: EffectTrust, Rule: trust.rule.clone(), Vulnerability: waived}
	default:
		return Decision{}
	}
}

// better returns the block and the trust entry that decide after it looks at
// e. It skips the checks that cannot change the result.
func better(e *entry, q Query, now time.Time, block, trust *entry) (*entry, *entry) {
	r := &e.rule
	switch r.Effect {
	case EffectBlock:
		if block != nil && compareEntries(e, block) >= 0 {
			return block, trust
		}
		if e.active(now) && e.matchesVersion(q.Package) {
			return e, trust
		}

	case EffectTrust:
		if block != nil || q.Verdict == VerdictVerifiedMalware {
			return block, trust
		}
		if trust != nil && compareEntries(e, trust) >= 0 {
			return block, trust
		}
		if waives(r, q) && e.active(now) && e.matchesVersion(q.Package) {
			return block, e
		}

	default:
	}

	return block, trust
}

// waives reports whether the scopes of a trust entry cover the control of the
// query. ScopeNamedVulnerability is not a control, so a query that asks with
// it gets no trust entry.
func waives(r *Rule, q Query) bool {
	if q.Control == ScopeNamedVulnerability {
		return false
	}

	if slices.Contains(r.Scopes, q.Control) {
		return true
	}

	_, ok := namedVulnerability(r, q)
	return ok
}

// namedVulnerability returns the first vulnerability of a trust entry with
// ScopeNamedVulnerability that a vulnerability query names.
func namedVulnerability(r *Rule, q Query) (WaivedVulnerability, bool) {
	if q.Control != ScopeVulnerability || !slices.Contains(r.Scopes, ScopeNamedVulnerability) {
		return WaivedVulnerability{}, false
	}

	for _, v := range r.Vulnerabilities {
		for _, id := range q.VulnerabilityIDs {
			if id != "" && strings.EqualFold(id, v.ID) {
				return v, true
			}
		}
	}

	return WaivedVulnerability{}, false
}

func (e *entry) active(now time.Time) bool {
	return e.rule.ExpiresAt.IsZero() || now.Before(e.rule.ExpiresAt)
}

// matchesVersion fails closed: when the version cannot be checked, a block
// entry matches and a trust entry does not.
func (e *entry) matchesVersion(p Package) bool {
	block := e.rule.Effect == EffectBlock

	switch {
	case e.version != "":
		if p.Version() == "" {
			return block
		}
		if p.Version() == e.version {
			return true
		}
		if !block {
			return false
		}

		// An error does not make the block match here. An ecosystem with no
		// version order, such as GitHub Actions, fails every comparison, and
		// a block on v4 would then block v3 too.
		cmp, err := p.CompareVersion(e.version)
		return err == nil && cmp == 0

	case e.rng != nil:
		ok, err := e.rng.Contains(p)
		if err != nil {
			return block
		}
		return ok

	default:
		return true
	}
}
