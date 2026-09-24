package pb

import (
	"fmt"
	"net/url"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
)

// PackageVersion is the identity of one package version. It is the only way
// to obtain a canonical name and version, so a function that takes it cannot
// receive an unfolded pair. The value is immutable. It keeps the raw spelling
// next to the canonical one: the raw form serves display, registry URLs and a
// re-fold when a rule changes, and the canonical form serves every comparison.
//
// The value is not comparable. Compare with Equal and index a map with Key.
// A == on two values would compare the raw spelling too, and a map keyed on
// the value would miss the entry the same identity wrote under another
// spelling.
type PackageVersion struct {
	_ [0]func()

	ecosystem     packagev1.Ecosystem
	rawName       string
	rawVersion    string
	name          string
	version       string
	ruleVersion   int
	versionParsed bool
}

// NewPackageVersion folds a proto under the rule of its ecosystem. It never
// fails. A nil message yields an unspecified ecosystem and empty strings.
func NewPackageVersion(pv *packagev1.PackageVersion) PackageVersion {
	return NewPackageVersionFromParts(pv.GetPackage().GetEcosystem(), pv.GetPackage().GetName(), pv.GetVersion())
}

// NewPackageVersionFromParts folds a raw name and version under the rule of
// the ecosystem. It never fails. A version the rule cannot parse stays raw and
// reports VersionParsed false, so a malformed input matches only itself.
func NewPackageVersionFromParts(ecosystem packagev1.Ecosystem, name, version string) PackageVersion {
	rule := ruleFor(ecosystem)
	canonicalName, canonicalVersion, parsed := rule.fold(name, version)

	return PackageVersion{
		ecosystem:     ecosystem,
		rawName:       name,
		rawVersion:    version,
		name:          canonicalName,
		version:       canonicalVersion,
		ruleVersion:   rule.version,
		versionParsed: parsed,
	}
}

// NewPackageVersionFromPurl parses a Package URL and folds it. It returns an
// error for a malformed PURL and for a PURL type the ecosystem enum cannot
// represent, because an identity that has lost its ecosystem and namespace
// would let two distinct packages share a key. The raw name and raw version
// are the coordinates as written in the PURL, percent-decoded, so the value
// folds the same way as one built from the parts.
func NewPackageVersionFromPurl(purl string) (PackageVersion, error) {
	p, err := parsePurl(purl)
	if err != nil {
		return PackageVersion{}, err
	}

	ecosystem := purlMapEcosystem(p.Type)
	if ecosystem == packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED {
		return PackageVersion{}, fmt.Errorf("unsupported purl type: %q", p.Type)
	}

	// PyPI, Cargo, RubyGems, NuGet and the editor extension types name a
	// package without a namespace. The name mapping would drop one, so the
	// constructor rejects it rather than yield a different package.
	name := purlIdentityName(ecosystem, p)
	if p.Namespace != "" && name == p.Name {
		return PackageVersion{}, fmt.Errorf("%s purl has a namespace: %q", p.Type, p.Namespace)
	}

	return NewPackageVersionFromParts(ecosystem, name, p.Version), nil
}

// Ecosystem is the ecosystem the name and version belong to.
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

// RawName is the name as the producer observed it.
func (p PackageVersion) RawName() string {
	return p.rawName
}

// RawVersion is the version as the producer observed it.
func (p PackageVersion) RawVersion() string {
	return p.rawVersion
}

// RuleVersion is the version of the rule that produced the canonical form.
func (p PackageVersion) RuleVersion() int {
	return p.ruleVersion
}

// HasRule reports whether the ecosystem has any fold rule.
func (p PackageVersion) HasRule() bool {
	return ruleFor(p.ecosystem).hasRule()
}

// HasVersionRule reports whether the ecosystem has a version rule. Count
// VersionParsed misses only where this is true.
func (p PackageVersion) HasVersionRule() bool {
	return ruleFor(p.ecosystem).hasVersionRule()
}

// VersionParsed reports whether a version rule parsed the raw version. It is
// false for every ecosystem with no version rule, where the raw version is
// the identity, so false alone does not mean the version is invalid. Read it
// with HasVersionRule.
func (p PackageVersion) VersionParsed() bool {
	return p.versionParsed
}

// CanonicalProto returns a new message that carries the canonical form. It
// serves a store that keeps canonical columns. A client that sends a request
// uses RawProto instead. A caller that mutates the message changes nothing
// inside the value.
func (p PackageVersion) CanonicalProto() *packagev1.PackageVersion {
	return newPackageVersionProto(p.ecosystem, p.name, p.version)
}

// RawProto returns a new message that carries the raw spelling. This is the
// form a client sends on the wire, so the server can fold it under its own
// rule and count how often the fold changes an identity.
func (p PackageVersion) RawProto() *packagev1.PackageVersion {
	return newPackageVersionProto(p.ecosystem, p.rawName, p.rawVersion)
}

// URN is the canonical Package URL. It fails for an ecosystem with no PURL
// type, and for a name a PURL cannot hold, rather than fabricate one. A URN
// parses back through NewPackageVersionFromPurl to the same identity.
func (p PackageVersion) URN() (string, error) {
	return identityPurl(p.ecosystem, p.name, p.version)
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
