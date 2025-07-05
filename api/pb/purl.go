package pb

import (
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

func NewPurlPackageVersion(purl string) (*purlPackageVersionHelper, error) {
	p, err := packageurl.FromString(purl)
	if err != nil {
		return nil, fmt.Errorf("invalid purl: %v", err)
	}

	ecosystem := purlMapEcosystem(p.Type)
	name := purlMapName(ecosystem, p)

	pv := &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: ecosystem,
			Name:      name,
		},
		Version: p.Version,
	}

	return &purlPackageVersionHelper{pv: pv}, nil
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
	case packagev1.Ecosystem_ECOSYSTEM_GO, packagev1.Ecosystem_ECOSYSTEM_NPM:
		return purl.Namespace + "/" + purl.Name
	case packagev1.Ecosystem_ECOSYSTEM_MAVEN:
		return purl.Namespace + ":" + purl.Name
	case packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS:
		return purl.Namespace + "/" + purl.Name
	default:
		return purl.Name
	}
}
