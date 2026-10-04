package pb

import (
	"errors"
	"fmt"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/google/osv-scalibr/semantic"
)

var (
	// ErrNoVersionOrder means the ecosystem has no version order, for example
	// a GitHub Actions ref, which is a tag, a branch or a commit.
	ErrNoVersionOrder = errors.New("ecosystem has no version order")

	// ErrDifferentPackages means the two values do not name one package, so
	// the order of their versions has no meaning.
	ErrDifferentPackages = errors.New("versions of different packages")
)

// osvEcosystems maps each ecosystem with a version order to the OSV ecosystem
// whose order applies. dry uses the OSV order because OSV holds the affected
// ranges that the vulnerability data comes from. An order that differs from
// OSV would put a version on the wrong side of a fix. Terraform, VS Code and
// Open VSX have no OSV ecosystem. Their versions are semver, so they use the
// npm order.
var osvEcosystems = map[packagev1.Ecosystem]string{
	packagev1.Ecosystem_ECOSYSTEM_MAVEN:              "Maven",
	packagev1.Ecosystem_ECOSYSTEM_NPM:                "npm",
	packagev1.Ecosystem_ECOSYSTEM_PYPI:               "PyPI",
	packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:           "RubyGems",
	packagev1.Ecosystem_ECOSYSTEM_NUGET:              "NuGet",
	packagev1.Ecosystem_ECOSYSTEM_CARGO:              "crates.io",
	packagev1.Ecosystem_ECOSYSTEM_GO:                 "Go",
	packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:          "Packagist",
	packagev1.Ecosystem_ECOSYSTEM_PUB:                "Pub",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM:          "npm",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_MODULE:   "npm",
	packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER: "npm",
	packagev1.Ecosystem_ECOSYSTEM_VSCODE:             "npm",
	packagev1.Ecosystem_ECOSYSTEM_OPENVSX:            "npm",
}

// CompareVersions orders two versions under the rule of the ecosystem. It
// returns -1, 0 or +1. The order is total: like OSV when it matches an
// affected range, it orders a string that the registry would reject, for
// example a PyPI legacy version. It returns ErrNoVersionOrder for an
// ecosystem with no order, and an error for an empty version. A caller that
// gets an error must not claim an order, for example an upgrade or a
// downgrade.
func CompareVersions(ecosystem packagev1.Ecosystem, a, b string) (int, error) {
	osvEcosystem, ok := osvEcosystems[ecosystem]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrNoVersionOrder, ecosystem)
	}
	if a == "" || b == "" {
		return 0, errors.New("cannot order an empty version")
	}

	va, err := semantic.Parse(a, osvEcosystem)
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
	_, ok := osvEcosystems[ecosystem]
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
