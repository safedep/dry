package pb

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVersionAgreesWithCompareVersions(t *testing.T) {
	cases := []struct {
		ecosystem packagev1.Ecosystem
		a, b      string
	}{
		{packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0rc1", "1.0"},
		{packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0.0.0", "1"},
		{packagev1.Ecosystem_ECOSYSTEM_PYPI, "2.0.post1", "2.0"},
		{packagev1.Ecosystem_ECOSYSTEM_MAVEN, "1.0-SNAPSHOT", "1.0"},
		{packagev1.Ecosystem_ECOSYSTEM_NUGET, "4.2.0.1", "4.2.0.2"},
		{packagev1.Ecosystem_ECOSYSTEM_NPM, "1.9.0", "1.10.0"},
		{packagev1.Ecosystem_ECOSYSTEM_NPM, "2.0.0-rc.1", "2.0.0"},
		{packagev1.Ecosystem_ECOSYSTEM_GO, "v1.2.3", "v1.2.3"},
		{packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, "1.0.0.pre", "1.0.0"},
		{packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, "v2.0-beta", "v2.0-RC-1"},
		{packagev1.Ecosystem_ECOSYSTEM_PUB, "1.0.0+build", "1.0.0"},
	}

	for _, tc := range cases {
		t.Run(tc.ecosystem.String()+" "+tc.a+" "+tc.b, func(t *testing.T) {
			want, err := CompareVersions(tc.ecosystem, tc.a, tc.b)
			require.NoError(t, err)

			a, err := ParseVersion(tc.ecosystem, tc.a)
			require.NoError(t, err)
			b, err := ParseVersion(tc.ecosystem, tc.b)
			require.NoError(t, err)

			got, err := a.Compare(b)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestParseVersionFails(t *testing.T) {
	_, err := ParseVersion(packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, "v1")
	assert.ErrorIs(t, err, ErrNoVersionOrder)

	_, err = ParseVersion(packagev1.Ecosystem_ECOSYSTEM_NPM, "")
	assert.Error(t, err)
}

func TestOrderedVersionCompareFails(t *testing.T) {
	npm, err := ParseVersion(packagev1.Ecosystem_ECOSYSTEM_NPM, "1.0.0")
	require.NoError(t, err)
	cargo, err := ParseVersion(packagev1.Ecosystem_ECOSYSTEM_CARGO, "1.0.0")
	require.NoError(t, err)

	_, err = npm.Compare(cargo)
	assert.Error(t, err, "two ecosystems")

	_, err = npm.Compare(OrderedVersion{})
	assert.Error(t, err, "zero value")

	_, err = OrderedVersion{}.Compare(npm)
	assert.Error(t, err, "zero value")
}
