package pb

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOSVEcosystemNamesArePinned pins every OSV name, because OSV data and
// OSV queries hold them. A wrong name silently loses an ecosystem.
func TestOSVEcosystemNamesArePinned(t *testing.T) {
	want := map[packagev1.Ecosystem]string{
		packagev1.Ecosystem_ECOSYSTEM_MAVEN:          "Maven",
		packagev1.Ecosystem_ECOSYSTEM_NPM:            "npm",
		packagev1.Ecosystem_ECOSYSTEM_PYPI:           "PyPI",
		packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS:       "RubyGems",
		packagev1.Ecosystem_ECOSYSTEM_NUGET:          "NuGet",
		packagev1.Ecosystem_ECOSYSTEM_CARGO:          "crates.io",
		packagev1.Ecosystem_ECOSYSTEM_GO:             "Go",
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS: "GitHub Actions",
		packagev1.Ecosystem_ECOSYSTEM_PACKAGIST:      "Packagist",
		packagev1.Ecosystem_ECOSYSTEM_VSCODE:         "VSCode",
		packagev1.Ecosystem_ECOSYSTEM_PUB:            "Pub",
		packagev1.Ecosystem_ECOSYSTEM_OPENVSX:        "VSCode:https://open-vsx.org",
	}

	got := map[packagev1.Ecosystem]string{}
	for _, ecosystem := range OSVEcosystems() {
		name, err := OSVEcosystemName(ecosystem)
		require.NoError(t, err)
		got[ecosystem] = name
	}
	assert.Equal(t, want, got)
}

// TestEveryEcosystemHasAnOSVDecision fails when safedep/api adds an
// ecosystem and nobody has decided whether OSV publishes it. Add the new
// value to osvEcosystemNames, or to noOSVEcosystem below.
func TestEveryEcosystemHasAnOSVDecision(t *testing.T) {
	noOSVEcosystem := map[packagev1.Ecosystem]bool{
		packagev1.Ecosystem_ECOSYSTEM_TERRAFORM:                       true,
		packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_MODULE:                true,
		packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER:              true,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY:               true,
		packagev1.Ecosystem_ECOSYSTEM_HOMEBREW:                        true,
		packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY:               true,
		packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY:            true,
		packagev1.Ecosystem_ECOSYSTEM_GOOGLE_CHROME_BROWSER_EXTENSION: true,
		packagev1.Ecosystem_ECOSYSTEM_FIREFOX_BROWSER_EXTENSION:       true,
	}

	for value := range packagev1.Ecosystem_name {
		ecosystem := packagev1.Ecosystem(value)
		if ecosystem == packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED {
			continue
		}
		t.Run(ecosystem.String(), func(t *testing.T) {
			name, err := OSVEcosystemName(ecosystem)
			if noOSVEcosystem[ecosystem] {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err, "decide whether OSV publishes this ecosystem")

			back, err := EcosystemFromOSVName(name)
			require.NoError(t, err)
			assert.Equal(t, ecosystem, back)
		})
	}
}

func TestOSVEcosystemName(t *testing.T) {
	t.Run("unspecified has no OSV name", func(t *testing.T) {
		_, err := OSVEcosystemName(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED)
		assert.Error(t, err)
	})

	t.Run("a value outside the enum has no OSV name", func(t *testing.T) {
		_, err := OSVEcosystemName(packagev1.Ecosystem(1000))
		assert.Error(t, err)
	})
}

func TestEcosystemFromOSVName(t *testing.T) {
	cases := []struct {
		name string
		want packagev1.Ecosystem
	}{
		{"PyPI", packagev1.Ecosystem_ECOSYSTEM_PYPI},
		{"crates.io", packagev1.Ecosystem_ECOSYSTEM_CARGO},
		{"GitHub Actions", packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS},
		{"VSCode", packagev1.Ecosystem_ECOSYSTEM_VSCODE},
		{"VSCode:https://open-vsx.org", packagev1.Ecosystem_ECOSYSTEM_OPENVSX},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := EcosystemFromOSVName(test.name)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}

	// pypi and cargo are SafeDep names, not OSV names. Debian:12 is an OSV
	// ecosystem with a suffix that no SafeDep ecosystem has. OSV writes the
	// registry URL with no trailing slash.
	for _, name := range []string{
		"", "pypi", "cargo", "github-actions", "Debian:12", "Debian", " npm",
		"VSCode:https://open-vsx.org/", "Maven:https://maven.google.com",
	} {
		t.Run("unknown "+name, func(t *testing.T) {
			got, err := EcosystemFromOSVName(name)
			assert.Error(t, err)
			assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, got)
		})
	}
}

func TestOSVEcosystemsAreInEnumOrder(t *testing.T) {
	ecosystems := OSVEcosystems()
	require.Len(t, ecosystems, len(osvEcosystemNames))
	assert.IsIncreasing(t, ecosystems)
}
