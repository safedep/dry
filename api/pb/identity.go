package pb

import (
	"fmt"
	"net/url"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// IdentityRuleVersion reports the version of the fold rule for an ecosystem.
// Zero means no rule: the fold is the identity function and HasRule is false.
// Bump one ecosystem's version when its rule changes. The owner of a store
// re-folds the rows written under the older version, and FoldWithRule lets a
// reader match those rows during the transition.
//
// Version 1 rules:
//   - PyPI: PEP 503 name, PEP 440 canonical version.
//   - RubyGems, Cargo, Packagist: lower-case name, raw version.
//
// Every other ecosystem, npm included, has no rule. npm keeps case because
// JSONStream and jsonstream are two packages on the registry.
func IdentityRuleVersion(ecosystem packagev1.Ecosystem) int {
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_PYPI,
		packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS,
		packagev1.Ecosystem_ECOSYSTEM_CARGO,
		packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:
		return 1
	default:
		return 0
	}
}

// Fold is the result of one fold rule applied to a name and a version.
type Fold struct {
	Name    string
	Version string

	// VersionParsed is true when a version rule parsed the input. It is
	// false when the ecosystem has no version rule, and false when the rule
	// could not parse the input, in which case Version holds the raw string.
	VersionParsed bool
}

// FoldWithRule folds a name and a version under one rule version of an
// ecosystem. Version 0 is the identity fold for every ecosystem. The fold
// never fails for a version that exists. A reader that has to match rows
// written under the previous rule folds the request under that version too.
func FoldWithRule(ecosystem packagev1.Ecosystem, ruleVersion int, name, version string) (Fold, error) {
	if ruleVersion == 0 {
		return Fold{Name: name, Version: version}, nil
	}

	if ruleVersion != IdentityRuleVersion(ecosystem) {
		return Fold{}, fmt.Errorf("no identity rule version %d for %s", ruleVersion, ecosystem)
	}

	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_PYPI:
		canonical, parsed := canonicalPypiVersion(version)
		return Fold{Name: CanonicalPackageName(ecosystem, name), Version: canonical, VersionParsed: parsed}, nil
	default:
		return Fold{Name: CanonicalPackageName(ecosystem, name), Version: version}, nil
	}
}

// PackageVersion is the identity of one package version. It is the only way
// to obtain a canonical name and version, so a function that takes it cannot
// receive an unfolded pair. The value is immutable. It keeps the raw spelling
// next to the canonical one: the raw form serves display, registry URLs and a
// re-fold when a rule changes, and the canonical form serves every comparison.
type PackageVersion struct {
	ecosystem     packagev1.Ecosystem
	rawName       string
	rawVersion    string
	name          string
	version       string
	ruleVersion   int
	versionParsed bool
}

// NewPackageVersion folds a proto under the current rule of its ecosystem. It
// never fails. A nil message yields an unspecified ecosystem and empty strings.
func NewPackageVersion(pv *packagev1.PackageVersion) PackageVersion {
	return NewPackageVersionFromParts(pv.GetPackage().GetEcosystem(), pv.GetPackage().GetName(), pv.GetVersion())
}

// NewPackageVersionFromParts folds a raw name and version under the current
// rule of the ecosystem. It never fails.
func NewPackageVersionFromParts(ecosystem packagev1.Ecosystem, name, version string) PackageVersion {
	ruleVersion := IdentityRuleVersion(ecosystem)
	folded, err := FoldWithRule(ecosystem, ruleVersion, name, version)
	if err != nil {
		// The current rule version always exists. A failure here is a bug in
		// IdentityRuleVersion, and the identity fold is the safe answer.
		folded = Fold{Name: name, Version: version}
		ruleVersion = 0
	}

	return PackageVersion{
		ecosystem:     ecosystem,
		rawName:       name,
		rawVersion:    version,
		name:          folded.Name,
		version:       folded.Version,
		ruleVersion:   ruleVersion,
		versionParsed: folded.VersionParsed,
	}
}

// NewPackageVersionFromPurl parses a Package URL and folds it. It returns an
// error only for a malformed PURL. The raw name is the name the PURL parser
// yields, which for a Go module path has already lost its case.
func NewPackageVersionFromPurl(purl string) (PackageVersion, error) {
	helper, err := NewPurlPackageVersion(purl)
	if err != nil {
		return PackageVersion{}, err
	}

	return NewPackageVersion(helper.PackageVersion()), nil
}

func (p PackageVersion) Ecosystem() packagev1.Ecosystem {
	return p.ecosystem
}

// Name is the canonical name.
func (p PackageVersion) Name() string {
	return p.name
}

// Version is the canonical version, or the raw version when no rule parsed it.
func (p PackageVersion) Version() string {
	return p.version
}

func (p PackageVersion) RawName() string {
	return p.rawName
}

func (p PackageVersion) RawVersion() string {
	return p.rawVersion
}

// RuleVersion is the version of the rule that produced the canonical form.
func (p PackageVersion) RuleVersion() int {
	return p.ruleVersion
}

// HasRule reports whether the ecosystem has any fold rule.
func (p PackageVersion) HasRule() bool {
	return p.ruleVersion != 0
}

// HasVersionRule reports whether the ecosystem has a version rule. Count
// VersionParsed misses only where this is true.
func (p PackageVersion) HasVersionRule() bool {
	return p.ecosystem == packagev1.Ecosystem_ECOSYSTEM_PYPI && p.ruleVersion != 0
}

// VersionParsed reports whether a version rule parsed the raw version.
func (p PackageVersion) VersionParsed() bool {
	return p.versionParsed
}

// Proto returns a new message that carries the canonical form. A caller that
// mutates it changes nothing inside the value.
func (p PackageVersion) Proto() *packagev1.PackageVersion {
	return newPackageVersionProto(p.ecosystem, p.name, p.version)
}

// RawProto returns a new message that carries the raw spelling. This is the
// form a client sends on the wire, so the server can fold it under its own
// rule and count how often the fold changes an identity.
func (p PackageVersion) RawProto() *packagev1.PackageVersion {
	return newPackageVersionProto(p.ecosystem, p.rawName, p.rawVersion)
}

// URN is the canonical Package URL. It fails for an ecosystem with no PURL
// type, as Purl does, rather than fabricate one.
func (p PackageVersion) URN() (string, error) {
	return Purl(p.Proto())
}

// Key is a string for maps and caches. It includes the rule version, so an
// entry written under an older rule never matches a lookup under a newer one.
// The name and the version are query-escaped, so a separator inside either
// one cannot make two distinct identities share a key. The format is stable,
// because a persistent cache stores it.
func (p PackageVersion) Key() string {
	return fmt.Sprintf("%s/%d/%s@%s", p.ecosystem.String(), p.ruleVersion,
		url.QueryEscape(p.name), url.QueryEscape(p.version))
}

// Equal compares the canonical forms of two values under the same rule.
func (p PackageVersion) Equal(other PackageVersion) bool {
	return p.ecosystem == other.ecosystem &&
		p.ruleVersion == other.ruleVersion &&
		p.name == other.name &&
		p.version == other.version
}

func newPackageVersionProto(ecosystem packagev1.Ecosystem, name, version string) *packagev1.PackageVersion {
	return &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: ecosystem,
			Name:      name,
		},
		Version: version,
	}
}
