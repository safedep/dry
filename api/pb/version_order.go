package pb

import (
	"errors"
	"fmt"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/safedep/dry/api/pb/internal/semantic"
)

var (
	// ErrNoVersionOrder means the ecosystem has no version order, for example
	// a GitHub Actions ref, which is a tag, a branch or a commit.
	ErrNoVersionOrder = errors.New("ecosystem has no version order")

	// ErrDifferentPackages means the two values do not name one package, so
	// the order of their versions has no meaning.
	ErrDifferentPackages = errors.New("versions of different packages")
)

type versionParser func(string) (semantic.Version, error)

func parser[V semantic.Version](parse func(string) V) versionParser {
	return func(s string) (semantic.Version, error) { return parse(s), nil }
}

// versionParsers holds the parser of each ecosystem with a version order. It
// is the OSV order, because the affected ranges of the vulnerability data come
// from OSV. An order that differs from OSV would put a version on the wrong
// side of a fix. npm, Cargo and Go share the semver order. Terraform, VS Code
// and Open VSX have no OSV ecosystem. Their versions are semver, so they use
// the semver order too.
var versionParsers = map[packagev1.Ecosystem]versionParser{
	packagev1.Ecosystem_ECOSYSTEM_MAVEN:     parser(semantic.ParseMavenVersion),
	packagev1.Ecosystem_ECOSYSTEM_NUGET:     parser(semantic.ParseNuGetVersion),
	packagev1.Ecosystem_ECOSYSTEM_PACKAGIST: parser(semantic.ParsePackagistVersion),
	packagev1.Ecosystem_ECOSYSTEM_PUB:       parser(semantic.ParsePubVersion),
	packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:  parser(semantic.ParseRubyGemsVersion),
	packagev1.Ecosystem_ECOSYSTEM_PYPI: func(s string) (semantic.Version, error) {
		return semantic.ParsePyPIVersion(s)
	},
	packagev1.Ecosystem_ECOSYSTEM_NPM:                parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_CARGO:              parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_GO:                 parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM:          parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_MODULE:   parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER: parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_VSCODE:             parser(semantic.ParseSemverVersion),
	packagev1.Ecosystem_ECOSYSTEM_OPENVSX:            parser(semantic.ParseSemverVersion),
}

// CompareVersions orders two versions under the rule of the ecosystem. It
// returns -1, 0 or +1. The order is total: like OSV when it matches an
// affected range, it orders a string that the registry would reject, for
// example a PyPI legacy version. It returns ErrNoVersionOrder for an
// ecosystem with no order, and an error for an empty version. A caller that
// gets an error must not claim an order, for example an upgrade or a
// downgrade.
func CompareVersions(ecosystem packagev1.Ecosystem, a, b string) (int, error) {
	parse, ok := versionParsers[ecosystem]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrNoVersionOrder, ecosystem)
	}
	if a == "" || b == "" {
		return 0, errors.New("cannot order an empty version")
	}

	va, err := parse(a)
	if err != nil {
		return 0, fmt.Errorf("parse %s version %q: %w", ecosystem, a, err)
	}

	order, err := va.CompareStr(b)
	if err != nil {
		return 0, fmt.Errorf("parse %s version %q: %w", ecosystem, b, err)
	}
	return order, nil
}

// HasVersionOrder reports whether the ecosystem has a version order.
func HasVersionOrder(ecosystem packagev1.Ecosystem) bool {
	_, ok := versionParsers[ecosystem]
	return ok
}

// Compare orders the version of p against the version of other. The two
// values must name one package under one rule, or Compare returns
// ErrDifferentPackages. It orders the raw versions, so a version that the
// identity rule could not parse still gets the order of the ecosystem.
func (p PackageVersion) Compare(other PackageVersion) (int, error) {
	if p.ecosystem != other.ecosystem || p.ruleVersion != other.ruleVersion || p.name != other.name {
		return 0, ErrDifferentPackages
	}
	return CompareVersions(p.ecosystem, p.rawVersion, other.rawVersion)
}
