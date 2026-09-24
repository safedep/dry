package pb

import (
	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// identityRule is the fold rule of one ecosystem: the version of the rule, the
// name fold, and the version fold when the ecosystem has one. The rule copies
// the registry's own notion of "the same package version", never an invented
// equivalence. Each fold function lives in the ecosystem's own file, and this
// table is the one place that says which ecosystem has which rule at which
// version. Bump the version in the same change as the fold it versions.
type identityRule struct {
	version     int
	foldName    func(string) string
	foldVersion func(string) (canonical string, parsed bool)
}

func (r identityRule) hasRule() bool {
	return r.version != 0
}

func (r identityRule) hasVersionRule() bool {
	return r.foldVersion != nil
}

func (r identityRule) fold(name, version string) (canonicalName, canonicalVersion string, parsed bool) {
	canonicalName = r.foldName(name)
	if r.foldVersion == nil {
		return canonicalName, version, false
	}

	canonicalVersion, parsed = r.foldVersion(version)
	return canonicalName, canonicalVersion, parsed
}

// identityRules holds every ecosystem with a rule. An ecosystem absent from
// the table has the identity rule at version 0: raw name, raw version.
//
// PyPI ships first. Every other ecosystem stays at version 0 until its
// conformance fixture under testdata/identity lands, with positive groups and
// at least one distinct pair taken from the registry. TestIdentityConformance
// fails on a rule with no such fixture. npm stays at version 0 on purpose:
// its registry is case-sensitive, so JSONStream and jsonstream are two
// packages with two artifacts, and a fold would merge them.
var identityRules = map[packagev1.Ecosystem]identityRule{
	packagev1.Ecosystem_ECOSYSTEM_PYPI: {version: 1, foldName: pep503Name, foldVersion: pep440Version},
}

var identityRuleNone = identityRule{foldName: func(name string) string { return name }}

func ruleFor(ecosystem packagev1.Ecosystem) identityRule {
	if rule, ok := identityRules[ecosystem]; ok {
		return rule
	}
	return identityRuleNone
}

// IdentityRuleVersion reports the version of the fold rule for an ecosystem.
// Zero means no rule. The owner of a store keeps the version next to the
// canonical columns, so a rule change can re-fold the rows written under the
// older version.
func IdentityRuleVersion(ecosystem packagev1.Ecosystem) int {
	return ruleFor(ecosystem).version
}
