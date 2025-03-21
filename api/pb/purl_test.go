package pb

import (
	"errors"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
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
