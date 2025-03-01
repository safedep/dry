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
