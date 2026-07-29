package pb

import (
	"errors"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/package-url/packageurl-go"
	"github.com/stretchr/testify/assert"
)

func TestPurlPackageVersionHelper(t *testing.T) {
	cases := []struct {
		name          string
		purl          string
		wantEcosystem packagev1.Ecosystem
		wantName      string
		wantVersion   string
		err           error
	}{
		{
			name:          "maven",
			purl:          "pkg:maven/org.apache.commons/compress@1.20",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_MAVEN,
			wantName:      "org.apache.commons:compress",
			wantVersion:   "1.20",
		},
		{
			name:          "go",
			purl:          "pkg:golang/github.com/golang/protobuf@v1.4.2",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_GO,
			wantName:      "github.com/golang/protobuf",
			wantVersion:   "v1.4.2",
		},
		{
			name:          "npm",
			purl:          "pkg:npm/@angular/core@12.0.0",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
			wantName:      "@angular/core",
			wantVersion:   "12.0.0",
		},
		{
			name:          "npm without scope",
			purl:          "pkg:npm/express@4.17.1",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
			wantName:      "express",
			wantVersion:   "4.17.1",
		},
		{
			name:          "github actions",
			purl:          "pkg:github/actions/setup-node@v2",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS,
			wantName:      "actions/setup-node",
			wantVersion:   "v2",
		},
		{
			name:          "ruby gems",
			purl:          "pkg:gem/rails@6.1.3",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS,
			wantName:      "rails",
			wantVersion:   "6.1.3",
		},
		{
			name:          "vscode extensions - vscode",
			purl:          "pkg:vscode/pub.ext@1.0.0",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_VSCODE,
			wantName:      "pub.ext",
			wantVersion:   "1.0.0",
		},
		{
			name:          "vscode extensions - vsx",
			purl:          "pkg:vsx/pub.ext@1.0.0",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_VSCODE,
			wantName:      "pub.ext",
			wantVersion:   "1.0.0",
		},
		{
			name:          "purl without version",
			purl:          "pkg:golang/github.com/golang/protobuf",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_GO,
			wantName:      "github.com/golang/protobuf",
			wantVersion:   "",
		},
		{
			name:          "openvsx extension - openvsx",
			purl:          "pkg:openvsx/castwide.solargraph@0.24.1",
			wantEcosystem: packagev1.Ecosystem_ECOSYSTEM_OPENVSX,
			wantName:      "castwide.solargraph",
			wantVersion:   "0.24.1",
		},
		{
			name: "invalid purl",
			purl: "pkg:invalid",
			err:  errors.New("invalid purl"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			h, err := NewPurlPackageVersion(test.purl)
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.wantEcosystem, h.Ecosystem())
				assert.Equal(t, test.wantName, h.Name())
				assert.Equal(t, test.wantVersion, h.Version())

				assert.Equal(t, test.wantEcosystem, h.PackageVersion().GetPackage().GetEcosystem())
				assert.Equal(t, test.wantName, h.PackageVersion().GetPackage().GetName())
				assert.Equal(t, test.wantVersion, h.PackageVersion().GetVersion())
			}
		})
	}
}

func TestPurlPackageVersionFromGithubUrl(t *testing.T) {
	cases := []struct {
		name        string
		githubUrl   string
		wantName    string
		wantVersion string
		err         error
	}{
		{
			name:        "github repository without branch",
			githubUrl:   "https://github.com/safedep/vet",
			wantName:    "safedep/vet",
			wantVersion: "",
		},
		{
			name:        "github repository with trailing slash",
			githubUrl:   "https://github.com/safedep/vet/",
			wantName:    "safedep/vet",
			wantVersion: "",
		},
		{
			name:        "github repository with branch",
			githubUrl:   "https://github.com/safedep/vet/tree/main",
			wantName:    "safedep/vet",
			wantVersion: "main",
		},
		{
			name:        "github repository with grouped branches",
			githubUrl:   "https://github.com/safedep/vet/tree/feat/branch",
			wantName:    "safedep/vet",
			wantVersion: "feat/branch",
		},
		{
			name:        "github repository with multi-grouped branches",
			githubUrl:   "https://github.com/safedep/vet/tree/feat/sub/branch",
			wantName:    "safedep/vet",
			wantVersion: "feat/sub/branch",
		},
		{
			name:        "github repository with tag",
			githubUrl:   "https://github.com/safedep/vet/tree/v1.0.0",
			wantName:    "safedep/vet",
			wantVersion: "v1.0.0",
		},
		{
			name:        "github repository with commit sha",
			githubUrl:   "https://github.com/safedep/vet/tree/5387a395a3b052670a35abfd937037963094d5b3",
			wantName:    "safedep/vet",
			wantVersion: "5387a395a3b052670a35abfd937037963094d5b3",
		},
		{
			name:        "github repository with short commit sha",
			githubUrl:   "https://github.com/safedep/vet/tree/5387a39",
			wantName:    "safedep/vet",
			wantVersion: "5387a39",
		},
		{
			name:        "github url with other tabs",
			githubUrl:   "https://github.com/safedep/vet/projects?query=is%3Aopen",
			wantName:    "safedep/vet",
			wantVersion: "",
		},
		{
			name:        "github url with fragments",
			githubUrl:   "https://github.com/safedep/vet#readme",
			wantName:    "safedep/vet",
			wantVersion: "",
		},
		{
			name:        "github repository with enterprise url",
			githubUrl:   "https://github.yourdomain.com/safedep/vet/tree/main",
			wantName:    "safedep/vet",
			wantVersion: "main",
		},
		{
			name:        "http protocol",
			githubUrl:   "http://github.com/safedep/vet",
			wantName:    "safedep/vet",
			wantVersion: "",
		},
		{
			name:      "invalid github url",
			githubUrl: "https://example.com/safedep/dry",
			err:       errors.New("invalid GitHub repository URL host"),
		},
		{
			name:      "invalid github url",
			githubUrl: "https://github.com",
			err:       errors.New("invalid GitHub repository URL format"),
		},
		{
			name:      "invalid github url",
			githubUrl: "https://githubcom/safedep/vet",
			err:       errors.New("invalid GitHub repository URL host"),
		},
		{
			name:      "invalid github url",
			githubUrl: "https://github.com/safedep/vet/blob/5387a395a3b052670a35abfd937037963094d5b3/api/exceptions_spec.proto",
			err:       errors.New("invalid GitHub repository URL format"),
		},
		{
			name:      "malformed url",
			githubUrl: "://github.com/safedep/vet",
			err:       errors.New("missing protocol scheme"),
		},
		{
			name:      "empty url",
			githubUrl: "",
			err:       errors.New("invalid GitHub repository URL host"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			h, err := NewPurlPackageVersionFromGithubUrl(test.githubUrl)
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY, h.Ecosystem())
				assert.Equal(t, test.wantName, h.Name())
				assert.Equal(t, test.wantVersion, h.Version())
			}
		})
	}
}

func TestEcosystemToPurlType(t *testing.T) {
	cases := []struct {
		name      string
		ecosystem packagev1.Ecosystem
		want      string
		wantErr   bool
	}{
		{"maven", packagev1.Ecosystem_ECOSYSTEM_MAVEN, packageurl.TypeMaven, false},
		{"go", packagev1.Ecosystem_ECOSYSTEM_GO, packageurl.TypeGolang, false},
		{"npm", packagev1.Ecosystem_ECOSYSTEM_NPM, packageurl.TypeNPM, false},
		{"nuget", packagev1.Ecosystem_ECOSYSTEM_NUGET, packageurl.TypeNuget, false},
		{"pypi", packagev1.Ecosystem_ECOSYSTEM_PYPI, packageurl.TypePyPi, false},
		{"rubygems", packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, packageurl.TypeGem, false},
		{"cargo", packagev1.Ecosystem_ECOSYSTEM_CARGO, packageurl.TypeCargo, false},
		{"packagist", packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, packageurl.TypeComposer, false},
		{"github actions", packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, packageurl.TypeGithub, false},
		{"github repository", packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY, packageurl.TypeGithub, false},
		{"vscode", packagev1.Ecosystem_ECOSYSTEM_VSCODE, "vscode", false},
		{"openvsx", packagev1.Ecosystem_ECOSYSTEM_OPENVSX, "openvsx", false},
		{"unspecified errors", packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, "", true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := EcosystemToPurlType(test.ecosystem)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.want, got)
		})
	}
}

func TestPurl(t *testing.T) {
	pv := func(ecosystem packagev1.Ecosystem, name, version string) *packagev1.PackageVersion {
		return &packagev1.PackageVersion{
			Package: &packagev1.Package{Ecosystem: ecosystem, Name: name},
			Version: version,
		}
	}

	cases := []struct {
		name    string
		pv      *packagev1.PackageVersion
		want    string
		wantErr bool
	}{
		{"npm unscoped", pv(packagev1.Ecosystem_ECOSYSTEM_NPM, "left-pad", "1.0.0"), "pkg:npm/left-pad@1.0.0", false},
		{"npm scoped", pv(packagev1.Ecosystem_ECOSYSTEM_NPM, "@angular/core", "17.0.0"), "pkg:npm/%40angular/core@17.0.0", false},
		{"pypi", pv(packagev1.Ecosystem_ECOSYSTEM_PYPI, "requests", "2.31.0"), "pkg:pypi/requests@2.31.0", false},
		// Maven names are stored as "group:artifact"; the namespace must split on
		// ":" (round-trips NewPurlPackageVersion) not "/".
		{"maven", pv(packagev1.Ecosystem_ECOSYSTEM_MAVEN, "org.apache.commons:compress", "1.20"), "pkg:maven/org.apache.commons/compress@1.20", false},
		{"go", pv(packagev1.Ecosystem_ECOSYSTEM_GO, "github.com/golang/protobuf", "v1.4.2"), "pkg:golang/github.com/golang/protobuf@v1.4.2", false},
		// GitHub Actions names are "owner/action"; GitHub repositories "owner/repo".
		// Both split the namespace on "/" and render under the "github" purl type.
		{"github actions", pv(packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, "actions/checkout", "v4"), "pkg:github/actions/checkout@v4", false},
		{"github repository", pv(packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY, "safedep/vet", "v1.0.0"), "pkg:github/safedep/vet@v1.0.0", false},
		// VSCode/OpenVSX have no namespace convention here, so the name is used verbatim.
		{"vscode", pv(packagev1.Ecosystem_ECOSYSTEM_VSCODE, "ms-python.python", "2024.0.0"), "pkg:vscode/ms-python.python@2024.0.0", false},
		{"no version", pv(packagev1.Ecosystem_ECOSYSTEM_PYPI, "requests", ""), "pkg:pypi/requests", false},
		{"unmapped ecosystem errors", pv(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, "x", "1"), "", true},
		{"empty name errors", pv(packagev1.Ecosystem_ECOSYSTEM_NPM, "", "1"), "", true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := Purl(test.pv)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.want, got)
		})
	}
}
