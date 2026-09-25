package pb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identityFixture is one ecosystem's golden conformance table. Every consumer
// repository runs the same table against the dry version it pins, so the rule
// cannot drift between repositories. The PyPI file is generated from the
// packaging reference implementation by scripts/identity-fixtures/pypi.py.
type identityFixture struct {
	Ecosystem   string `json:"ecosystem"`
	RuleVersion int    `json:"rule_version"`
	Source      string `json:"source"`

	Versions []identityVersionRow `json:"versions"`
	Names    []identityNameRow    `json:"names"`

	// Positive rows: every spelling in a group folds to one value.
	VersionGroups [][]string `json:"version_groups"`
	NameGroups    [][]string `json:"name_groups"`

	// Negative rows: the two spellings of a pair must stay distinct. A fold
	// that merges unrelated packages is idempotent too, so idempotence alone
	// proves nothing.
	VersionDistinct [][]string `json:"version_distinct"`
	NameDistinct    [][]string `json:"name_distinct"`
}

type identityVersionRow struct {
	Input     string `json:"input"`
	Canonical string `json:"canonical"`
	Parsed    bool   `json:"parsed"`
}

type identityNameRow struct {
	Input     string `json:"input"`
	Canonical string `json:"canonical"`
}

func TestIdentityConformance(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "identity", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	fixtures := map[packagev1.Ecosystem]identityFixture{}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			raw, err := os.ReadFile(file)
			require.NoError(t, err)

			var fixture identityFixture
			require.NoError(t, json.Unmarshal(raw, &fixture))

			ecosystemValue, ok := packagev1.Ecosystem_value[fixture.Ecosystem]
			require.True(t, ok, "unknown ecosystem %q", fixture.Ecosystem)
			ecosystem := packagev1.Ecosystem(ecosystemValue)
			fixtures[ecosystem] = fixture

			assert.Equal(
				t,
				fixture.RuleVersion,
				IdentityRuleVersion(ecosystem),
				"fixture rule version must match the code",
			)

			runIdentityRows(t, ecosystem, fixture)
			runIdentityGroups(t, ecosystem, fixture)
			runIdentityDistinct(t, ecosystem, fixture)
		})
	}

	// A rule is a claim about what the registry treats as one package
	// version. The claim ships with its evidence: positive rows and at least
	// one distinct pair from the registry. A rule with no fixture folds on
	// belief, which is how two packages become one row.
	t.Run("every rule has a fixture with a distinct pair", func(t *testing.T) {
		for ecosystem, rule := range identityRules {
			fixture, ok := fixtures[ecosystem]
			require.True(t, ok, "%s is at rule version %d and has no fixture", ecosystem, rule.version)
			assert.NotEmpty(t, fixture.NameGroups, "%s fixture has no positive name rows", ecosystem)
			assert.NotEmpty(t, fixture.NameDistinct, "%s fixture has no distinct name pair", ecosystem)
			if rule.hasVersionRule() {
				assert.NotEmpty(t, fixture.VersionGroups, "%s fixture has no positive version rows", ecosystem)
				assert.NotEmpty(t, fixture.VersionDistinct, "%s fixture has no distinct version pair", ecosystem)
			}
		}
	})
}

func runIdentityRows(t *testing.T, ecosystem packagev1.Ecosystem, fixture identityFixture) {
	t.Helper()

	for _, row := range fixture.Versions {
		t.Run("version "+row.Input, func(t *testing.T) {
			pv := NewPackageVersionFromParts(ecosystem, "", row.Input)
			assert.Equal(t, row.Canonical, pv.Version())
			assert.Equal(t, row.Parsed, pv.VersionParsed())

			again := NewPackageVersionFromParts(ecosystem, "", pv.Version())
			assert.Equal(t, pv.Version(), again.Version(), "folding a canonical version must be a no-op")
		})
	}

	for _, row := range fixture.Names {
		t.Run("name "+row.Input, func(t *testing.T) {
			pv := NewPackageVersionFromParts(ecosystem, row.Input, "")
			assert.Equal(t, row.Canonical, pv.Name())

			again := NewPackageVersionFromParts(ecosystem, pv.Name(), "")
			assert.Equal(t, pv.Name(), again.Name(), "folding a canonical name must be a no-op")
		})
	}
}

func runIdentityGroups(t *testing.T, ecosystem packagev1.Ecosystem, fixture identityFixture) {
	t.Helper()

	for _, group := range fixture.VersionGroups {
		require.NotEmpty(t, group)
		first := NewPackageVersionFromParts(ecosystem, "pkg", group[0])
		for _, spelling := range group[1:] {
			other := NewPackageVersionFromParts(ecosystem, "pkg", spelling)
			assert.True(t, first.Equal(other), "%q and %q must fold to one version", group[0], spelling)
		}
	}

	for _, group := range fixture.NameGroups {
		require.NotEmpty(t, group)
		first := NewPackageVersionFromParts(ecosystem, group[0], "1")
		for _, spelling := range group[1:] {
			other := NewPackageVersionFromParts(ecosystem, spelling, "1")
			assert.True(t, first.Equal(other), "%q and %q must fold to one name", group[0], spelling)
		}
	}
}

func runIdentityDistinct(t *testing.T, ecosystem packagev1.Ecosystem, fixture identityFixture) {
	t.Helper()

	for _, pair := range fixture.VersionDistinct {
		require.Len(t, pair, 2)
		a := NewPackageVersionFromParts(ecosystem, "pkg", pair[0])
		b := NewPackageVersionFromParts(ecosystem, "pkg", pair[1])
		assert.False(t, a.Equal(b), "%q and %q must stay two versions", pair[0], pair[1])
	}

	for _, pair := range fixture.NameDistinct {
		require.Len(t, pair, 2)
		a := NewPackageVersionFromParts(ecosystem, pair[0], "1")
		b := NewPackageVersionFromParts(ecosystem, pair[1], "1")
		assert.False(t, a.Equal(b), "%q and %q must stay two packages", pair[0], pair[1])
	}
}
