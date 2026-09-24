package pb

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/package-url/packageurl-go"
)

type purlPackageVersionHelper struct {
	pv *packagev1.PackageVersion
}

// NewPurlPackageVersion parses a purl into the proto form. Its output is
// frozen: existing callers store the names it returns and look them up again,
// so a change here before those callers move to PackageVersion would give one
// package two names. TestNewPurlPackageVersionIsFrozen pins the output. Use NewPackageVersionFromPurl to obtain an identity to
// compare, key or send.
func NewPurlPackageVersion(purl string) (*purlPackageVersionHelper, error) {
	p, err := packageurl.FromString(purl)
	if err != nil {
		return nil, fmt.Errorf("invalid purl: %v", err)
	}

	ecosystem := purlMapEcosystem(p.Type)
	pv := &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: ecosystem,
			Name:      purlMapName(ecosystem, p),
		},
		Version: p.Version,
	}

	return &purlPackageVersionHelper{pv: pv}, nil
}

// parsePurl parses a purl and keeps the namespace, name and version as
// written, percent-decoded. packageurl-go rewrites them for some types before
// any SafeDep rule runs: it lower-cases golang, github, bitbucket and
// composer, and folds pypi underscores. The identity rules own every fold,
// and a Go module path is case-sensitive, so the parser must not apply its
// own.
func parsePurl(purl string) (packageurl.PackageURL, error) {
	p, err := packageurl.FromString(purl)
	if err != nil {
		return packageurl.PackageURL{}, fmt.Errorf("invalid purl: %v", err)
	}

	var typ string
	typ, p.Namespace, p.Name, p.Version, err = purlObservedCoordinates(purl)
	if err != nil {
		return packageurl.PackageURL{}, fmt.Errorf("invalid purl: %v", err)
	}

	// In a pkg:// purl the parser resolves dot segments of the decoded path,
	// so pkg://npm/%2E%2E/pypi/x@1 reads as a pypi purl. A type that differs
	// from the one written would pair one ecosystem's rule with another's
	// coordinates.
	if !strings.EqualFold(typ, p.Type) {
		return packageurl.PackageURL{}, fmt.Errorf("invalid purl: type %q is written as %q", p.Type, typ)
	}

	return p, nil
}

// purlObservedCoordinates splits a purl as written, the way
// packageurl.FromString splits the opaque pkg:type/... form, before the
// parser adjusts the result for the type. For pkg:/ and pkg:// the parser
// splits the decoded path instead, so pkg://pypi/ns%2Frequests@2.0 would read
// as namespace ns; splitting the raw string keeps every spelling of a purl
// one identity. The caller has already validated the purl with the parser.
func purlObservedCoordinates(purl string) (typ, namespace, name, version string, err error) {
	_, rest, ok := strings.Cut(purl, ":")
	if !ok {
		return "", "", "", "", errors.New("purl is missing its scheme")
	}
	rest, _, _ = strings.Cut(rest, "#")
	rest, _, _ = strings.Cut(rest, "?")
	rest = strings.TrimLeft(rest, "/")

	typ, rest, ok = strings.Cut(rest, "/")
	if !ok {
		return "", "", "", "", errors.New("purl is missing type or name")
	}

	name = rest
	if i := strings.LastIndex(name, "/"); i >= 0 {
		if namespace, err = purlNamespace(name[:i]); err != nil {
			return "", "", "", "", err
		}
		name = name[i+1:]
	}

	if i := strings.LastIndex(name, "@"); i >= 0 {
		name, version = name[:i], name[i+1:]
		if version, err = url.PathUnescape(version); err != nil {
			return "", "", "", "", err
		}
	}

	if name, err = url.PathUnescape(name); err != nil {
		return "", "", "", "", err
	}

	return typ, namespace, name, version, nil
}

// purlNamespace decodes a raw namespace and drops its empty segments, as the
// purl spec requires. packageurl.FromString keeps them, so it reads
// github.com//Azure where ToString writes github.com/Azure, and an identity
// that kept them would not survive a round trip through URN.
func purlNamespace(raw string) (string, error) {
	segments := make([]string, 0, strings.Count(raw, "/")+1)
	for _, segment := range strings.Split(raw, "/") {
		if segment == "" {
			continue
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			return "", err
		}
		segments = append(segments, decoded)
	}
	return strings.Join(segments, "/"), nil
}

var githubHostRegexp = regexp.MustCompile(`^github(\.[a-zA-Z0-9-]+)?\.com$`)

func NewPurlPackageVersionFromGithubUrl(githubUrl string) (*purlPackageVersionHelper, error) {
	parsedUrl, err := url.Parse(githubUrl)
	if err != nil {
		return nil, err
	}

	if !githubHostRegexp.MatchString(parsedUrl.Host) {
		return nil, fmt.Errorf("invalid GitHub repository URL host")
	}

	parts := strings.Split(strings.Trim(parsedUrl.Path, "/"), "/")
	if len(parts) < 2 || (len(parts) > 3 && parts[2] != "tree") {
		return nil, fmt.Errorf("invalid GitHub repository URL format")
	}

	owner := parts[0]
	repo := parts[1]

	ref := ""
	if len(parts) > 3 {
		ref = strings.Join(parts[3:], "/")
	}

	pv := &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY,
			Name:      owner + "/" + repo,
		},
		Version: ref,
	}

	return &purlPackageVersionHelper{pv: pv}, nil
}

func (p *purlPackageVersionHelper) PackageVersion() *packagev1.PackageVersion {
	return p.pv
}

func (p *purlPackageVersionHelper) Ecosystem() packagev1.Ecosystem {
	return p.pv.Package.Ecosystem
}

func (p *purlPackageVersionHelper) Name() string {
	return p.pv.Package.Name
}

func (p *purlPackageVersionHelper) Version() string {
	return p.pv.Version
}

func purlMapEcosystem(ecosystem string) packagev1.Ecosystem {
	switch ecosystem {
	case packageurl.TypeMaven:
		return packagev1.Ecosystem_ECOSYSTEM_MAVEN
	case packageurl.TypeGolang, "go":
		return packagev1.Ecosystem_ECOSYSTEM_GO
	case packageurl.TypeNPM:
		return packagev1.Ecosystem_ECOSYSTEM_NPM
	case packageurl.TypeNuget:
		return packagev1.Ecosystem_ECOSYSTEM_NUGET
	case packageurl.TypePyPi, "pip":
		return packagev1.Ecosystem_ECOSYSTEM_PYPI
	case packageurl.TypeGem, "rubygems":
		return packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS
	case packageurl.TypeCargo:
		return packagev1.Ecosystem_ECOSYSTEM_CARGO
	case packageurl.TypeComposer:
		return packagev1.Ecosystem_ECOSYSTEM_PACKAGIST
	case packageurl.TypeGithub, "actions":
		return packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS
	case packageurl.TypeGitlab:
		return packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY
	case packageurl.TypeBitbucket:
		return packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY
	// https://github.com/package-url/purl-spec/issues/287
	case "vscode", "vsix", "vsx":
		return packagev1.Ecosystem_ECOSYSTEM_VSCODE
	case "openvsx":
		return packagev1.Ecosystem_ECOSYSTEM_OPENVSX
	default:
		return packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED
	}
}

// purlMapName joins the purl namespace and name the way NewPurlPackageVersion
// always has. It drops the Composer vendor. That is a defect, but the frozen
// helper keeps it until its writers move to PackageVersion.
func purlMapName(ecosystem packagev1.Ecosystem, purl packageurl.PackageURL) string {
	if purl.Namespace == "" {
		return purl.Name
	}

	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_GO, packagev1.Ecosystem_ECOSYSTEM_NPM:
		return purl.Namespace + "/" + purl.Name
	case packagev1.Ecosystem_ECOSYSTEM_MAVEN:
		return purl.Namespace + ":" + purl.Name
	case packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS,
		packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY:
		return purl.Namespace + "/" + purl.Name
	default:
		return purl.Name
	}
}

// purlIdentityName joins the namespace and name for PackageVersion. It keeps
// the Composer vendor, which purlMapName drops, so two vendors' packages of
// one name stay two identities and URN() round-trips.
func purlIdentityName(ecosystem packagev1.Ecosystem, purl packageurl.PackageURL) string {
	if ecosystem == packagev1.Ecosystem_ECOSYSTEM_PACKAGIST && purl.Namespace != "" {
		return purl.Namespace + "/" + purl.Name
	}
	return purlMapName(ecosystem, purl)
}

// CanonicalPackageName is the name fold that existing callers store today.
// Its output is frozen, for the same reason as NewPurlPackageVersion. The fold
// rules of PackageVersion live in identityRules and differ from this one for
// npm, which is case-sensitive. Callers move to PackageVersion, and the npm
// change lands with that move.
func CanonicalPackageName(ecosystem packagev1.Ecosystem, name string) string {
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_PYPI:
		return pep503Name(name)

	case packagev1.Ecosystem_ECOSYSTEM_NPM,
		packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS,
		packagev1.Ecosystem_ECOSYSTEM_CARGO,
		packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:
		return strings.ToLower(name)

	default:
		return name
	}
}

// EcosystemToPurlType maps a PackageVersion ecosystem to its canonical Package
// URL type. It is the build-side counterpart of purlMapEcosystem, but not a
// strict inverse: purlMapEcosystem collapses several purl types and aliases
// onto one ecosystem (e.g. "golang"/"go", "pypi"/"pip", and both a bare
// "github" and "actions" onto GITHUB_ACTIONS), and this helper maps both
// ECOSYSTEM_GITHUB_ACTIONS and ECOSYSTEM_GITHUB_REPOSITORY to the "github"
// type — so a round-trip through both is not guaranteed to be lossless.
// Ecosystems with no canonical purl type (including ECOSYSTEM_UNSPECIFIED)
// return an error so callers never fabricate a purl.
func EcosystemToPurlType(ecosystem packagev1.Ecosystem) (string, error) {
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_MAVEN:
		return packageurl.TypeMaven, nil
	case packagev1.Ecosystem_ECOSYSTEM_GO:
		return packageurl.TypeGolang, nil
	case packagev1.Ecosystem_ECOSYSTEM_NPM:
		return packageurl.TypeNPM, nil
	case packagev1.Ecosystem_ECOSYSTEM_NUGET:
		return packageurl.TypeNuget, nil
	case packagev1.Ecosystem_ECOSYSTEM_PYPI:
		return packageurl.TypePyPi, nil
	case packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:
		return packageurl.TypeGem, nil
	case packagev1.Ecosystem_ECOSYSTEM_CARGO:
		return packageurl.TypeCargo, nil
	case packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:
		return packageurl.TypeComposer, nil
	case packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY:
		return packageurl.TypeGithub, nil
	case packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY:
		return packageurl.TypeGitlab, nil
	case packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY:
		return packageurl.TypeBitbucket, nil
	case packagev1.Ecosystem_ECOSYSTEM_VSCODE:
		return "vscode", nil
	case packagev1.Ecosystem_ECOSYSTEM_OPENVSX:
		return "openvsx", nil
	default:
		return "", fmt.Errorf("no purl type for ecosystem: %s", ecosystem)
	}
}

// Purl builds a canonical Package URL string for a PackageVersion. It is the
// inverse of NewPurlPackageVersion: the ecosystem selects the purl type and the
// package name is split back into purl namespace/name using the same
// per-ecosystem convention purlMapName encodes (npm "@scope/name", maven
// "group:artifact", go/github "owner/repo"). It returns an error for an
// ecosystem with no purl type or an empty name rather than emitting a malformed
// purl.
//
// Its output is frozen, like NewPurlPackageVersion: existing callers build
// keys from it. TestPurlIsFrozen pins the output. PackageVersion
// builds its URN with identityPurl instead.
func Purl(pv *packagev1.PackageVersion) (string, error) {
	pkg := pv.GetPackage()
	name := pkg.GetName()
	if name == "" {
		return "", fmt.Errorf("cannot build purl: empty package name")
	}

	purlType, err := EcosystemToPurlType(pkg.GetEcosystem())
	if err != nil {
		return "", err
	}

	namespace, shortName := purlSplitName(pkg.GetEcosystem(), name)
	return packageurl.NewPackageURL(purlType, namespace, shortName, pv.GetVersion(), nil, "").ToString(), nil
}

// identityPurl builds the purl of a PackageVersion. It splits the name with
// purlIdentitySplitName, the inverse of purlIdentityName, so the Composer
// vendor becomes the namespace. It returns an error for a name a purl cannot
// hold, so every URN parses back to the identity that built it.
func identityPurl(ecosystem packagev1.Ecosystem, name, version string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("cannot build purl: empty package name")
	}

	purlType, err := EcosystemToPurlType(ecosystem)
	if err != nil {
		return "", err
	}

	namespace, shortName := purlIdentitySplitName(ecosystem, name)
	if !purlRepresentable(name, namespace, shortName) {
		return "", fmt.Errorf("cannot build purl: package name %q has an empty namespace segment or name", name)
	}
	return packageurl.NewPackageURL(purlType, namespace, shortName, version, nil, "").ToString(), nil
}

// purlRepresentable reports whether a purl can hold the split of a name.
// ToString drops empty namespace segments, so the names a//b and a/b, and the
// Maven names :a and a, would each render one purl, and a/ would render a purl
// with no name.
func purlRepresentable(name, namespace, shortName string) bool {
	if shortName == "" {
		return false
	}
	if namespace == "" {
		return shortName == name
	}
	for _, segment := range strings.Split(namespace, "/") {
		if segment == "" {
			return false
		}
	}
	return true
}

// purlSplitName is the inverse of purlMapName: it splits a safedep package name
// back into the purl namespace and name for the ecosystem's convention. A name
// with no namespace separator yields an empty namespace.
func purlSplitName(ecosystem packagev1.Ecosystem, name string) (string, string) {
	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_MAVEN:
		if i := strings.LastIndex(name, ":"); i >= 0 {
			return name[:i], name[i+1:]
		}
	case packagev1.Ecosystem_ECOSYSTEM_GO,
		packagev1.Ecosystem_ECOSYSTEM_NPM,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY:
		if i := strings.LastIndex(name, "/"); i >= 0 {
			return name[:i], name[i+1:]
		}
	}
	return "", name
}

// purlIdentitySplitName is the inverse of purlIdentityName. It differs from
// purlSplitName only for Packagist, whose vendor is the purl namespace.
func purlIdentitySplitName(ecosystem packagev1.Ecosystem, name string) (string, string) {
	if ecosystem == packagev1.Ecosystem_ECOSYSTEM_PACKAGIST {
		if i := strings.LastIndex(name, "/"); i >= 0 {
			return name[:i], name[i+1:]
		}
		return "", name
	}
	return purlSplitName(ecosystem, name)
}
