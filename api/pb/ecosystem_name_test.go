package pb

import (
	"regexp"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEcosystemNamesArePinned pins every name, because configs and policies
// hold them. A change to this table is a breaking change for every tool.
func TestEcosystemNamesArePinned(t *testing.T) {
	assert.Equal(t, []string{
		"maven", "npm", "pypi", "rubygems", "nuget", "cargo", "go", "github-actions", "packagist",
		"terraform", "terraform-module", "terraform-provider", "vscode", "github-repository", "openvsx",
		"homebrew", "gitlab-repository", "bitbucket-repository", "pub",
	}, EcosystemNames())
}

// TestEveryEcosystemHasAName fails when safedep/api adds an ecosystem and
// dry has no name for it yet.
func TestEveryEcosystemHasAName(t *testing.T) {
	format := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for value := range packagev1.Ecosystem_name {
		ecosystem := packagev1.Ecosystem(value)
		if ecosystem == packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED {
			continue
		}
		t.Run(ecosystem.String(), func(t *testing.T) {
			name, err := EcosystemName(ecosystem)
			require.NoError(t, err)
			assert.Regexp(t, format, name)

			back, err := EcosystemFromName(name)
			require.NoError(t, err)
			assert.Equal(t, ecosystem, back)
		})
	}
}

func TestEcosystemName(t *testing.T) {
	t.Run("unspecified has no name", func(t *testing.T) {
		_, err := EcosystemName(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED)
		assert.Error(t, err)
	})

	t.Run("a value outside the enum has no name", func(t *testing.T) {
		_, err := EcosystemName(packagev1.Ecosystem(1000))
		assert.Error(t, err)
	})
}

func TestEcosystemFromName(t *testing.T) {
	cases := []struct {
		name string
		want packagev1.Ecosystem
	}{
		{"pypi", packagev1.Ecosystem_ECOSYSTEM_PYPI},
		{"PyPI", packagev1.Ecosystem_ECOSYSTEM_PYPI},
		{"GitHub-Actions", packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS},
		{"terraform-provider", packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER},
		{"pub", packagev1.Ecosystem_ECOSYSTEM_PUB},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := EcosystemFromName(test.name)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}

	for _, name := range []string{"", "golang", "ECOSYSTEM_PYPI", " pypi", "unspecified"} {
		t.Run("unknown "+name, func(t *testing.T) {
			got, err := EcosystemFromName(name)
			assert.Error(t, err)
			assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, got)
		})
	}
}
