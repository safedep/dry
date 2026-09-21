package pb

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/package-url/packageurl-go"
)

type purlPackageVersionHelper struct {
	pv *packagev1.PackageVersion
}

func NewPurlPackageVersion(purl string) (*purlPackageVersionHelper, error) {
	p, err := parsePurl(purl)
	if err != nil {
		return nil, err
	}

	ecosystem := purlMapEcosystem(p.Type)
	pv := &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: ecosystem,
			Name:      CanonicalPackageName(ecosystem, purlMapName(ecosystem, p)),
		},
		Version: p.Version,
	}

	return &purlPackageVersionHelper{pv: pv}, nil
}

// parsePurl parses a purl and keeps the namespace and name as written,
// percent-decoded. packageurl-go rewrites both for some types before any
// SafeDep rule runs: it lower-cases golang, github, bitbucket and composer,
// and folds pypi underscores. The identity rules own every fold, and a Go
// module path is case-sensitive, so the parser must not apply its own.
func parsePurl(purl string) (packageurl.PackageURL, error) {
	p, err := packageurl.FromString(purl)
	if err != nil {
		return packageurl.PackageURL{}, fmt.Errorf("invalid purl: %v", err)
	}

	p.Namespace, p.Name, err = purlObservedName(purl)
	if err != nil {
		return packageurl.PackageURL{}, fmt.Errorf("invalid purl: %v", err)
	}

	return p, nil
}

// purlObservedName splits a purl the way packageurl.FromString does, up to
// the point where the parser adjusts the result for the type. The steps
// mirror the parser so both agree on where the namespace, name and version
// start. The caller has already validated the purl with the parser.
func purlObservedName(purl string) (namespace, name string, err error) {
	u, err := url.Parse(purl)
	if err != nil {
		return "", "", err
	}

	rest := u.Opaque
	if rest == "" {
		rest = strings.TrimPrefix(path.Join(u.Host, u.Path), "/")
	}

	_, rest, ok := strings.Cut(rest, "/")
	if !ok {
		return "", "", errors.New("purl is missing type or name")
	}

	name = rest
	if i := strings.LastIndex(name, "/"); i >= 0 {
		namespace, name = name[:i], name[i+1:]
		if namespace, err = url.PathUnescape(namespace); err != nil {
			return "", "", err
		}
	}

	if i := strings.LastIndex(name, "@"); i >= 0 {
		name = name[:i]
	}

	if name, err = url.PathUnescape(name); err != nil {
		return "", "", err
	}

	return strings.Trim(namespace, "/"), name, nil
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

func purlMapName(ecosystem packagev1.Ecosystem, purl packageurl.PackageURL) string {
	if purl.Namespace == "" {
		return purl.Name
	}

	switch ecosystem {
	case packagev1.Ecosystem_ECOSYSTEM_GO,
		packagev1.Ecosystem_ECOSYSTEM_NPM,
		packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:
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

// CanonicalPackageName returns the one spelling of a package name for its
// ecosystem, so two producers that disagree on case or separators still name
// one package. The rule per ecosystem lives in identityRules. A registry that
// treats names case-sensitively, npm among them, and every ecosystem with no
// rule keep the raw name.
//
// packageurl-go typeAdjustName is close but does not fit: for PyPI it folds
// only `_`, and it lower-cases Go module paths, which are case-sensitive.
func CanonicalPackageName(ecosystem packagev1.Ecosystem, name string) string {
	return ruleFor(ecosystem).foldName(name)
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
		packagev1.Ecosystem_ECOSYSTEM_PACKAGIST,
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
