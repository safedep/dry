package pb

import (
	"bufio"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name      string
		ecosystem packagev1.Ecosystem
		a, b      string
		want      int
	}{
		{"pypi pre-release before release", packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0rc1", "1.0", -1},
		{"pypi post-release after release", packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0", "1.0.post1", -1},
		{"pypi pre-release of the next major", packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.9.5", "2.0.0rc2", -1},
		{"pypi trailing zeros are one release", packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0.0.0.0.0", "1", 0},
		{"nuget four parts", packagev1.Ecosystem_ECOSYSTEM_NUGET, "4.2.0.1", "4.2.0.2", -1},
		{"maven snapshot before release", packagev1.Ecosystem_ECOSYSTEM_MAVEN, "1.0-SNAPSHOT", "1.0", -1},
		{"maven qualifiers", packagev1.Ecosystem_ECOSYSTEM_MAVEN, "1.0.0.Final", "1.0.1", -1},
		{"npm numeric not lexical", packagev1.Ecosystem_ECOSYSTEM_NPM, "1.9.0", "1.10.0", -1},
		{"go numeric not lexical", packagev1.Ecosystem_ECOSYSTEM_GO, "v1.2.3", "v1.10.0", -1},
		{"go pseudo-version before release", packagev1.Ecosystem_ECOSYSTEM_GO, "v0.0.0-20230101000000-abcdefabcdef", "v0.1.0", -1},
		{"cargo", packagev1.Ecosystem_ECOSYSTEM_CARGO, "1.0.0-alpha", "1.0.0", -1},
		{"rubygems pre-release", packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, "1.0.0.pre", "1.0.0", -1},
		{"packagist", packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, "v2.0-beta", "v2.0-RC-1", -1},
		{"pub build metadata", packagev1.Ecosystem_ECOSYSTEM_PUB, "1.0.0", "1.0.0+build", -1},
		{"terraform provider", packagev1.Ecosystem_ECOSYSTEM_TERRAFORM_PROVIDER, "5.9.0", "5.31.0", -1},
		{"vscode", packagev1.Ecosystem_ECOSYSTEM_VSCODE, "2024.10.0", "2024.2.0", 1},
		{"equal", packagev1.Ecosystem_ECOSYSTEM_NPM, "1.0.0", "1.0.0", 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := CompareVersions(test.ecosystem, test.a, test.b)
			require.NoError(t, err)
			assert.Equal(t, test.want, got)

			back, err := CompareVersions(test.ecosystem, test.b, test.a)
			require.NoError(t, err)
			assert.Equal(t, -test.want, back, "the order must be antisymmetric")
		})
	}
}

func TestCompareVersionsHasNoOrder(t *testing.T) {
	for _, ecosystem := range []packagev1.Ecosystem{
		packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY,
		packagev1.Ecosystem_ECOSYSTEM_HOMEBREW,
	} {
		t.Run(ecosystem.String(), func(t *testing.T) {
			assert.False(t, HasVersionOrder(ecosystem))
			_, err := CompareVersions(ecosystem, "v1", "v2")
			assert.ErrorIs(t, err, ErrNoVersionOrder)
		})
	}
}

func TestCompareVersionsRejectsBadInput(t *testing.T) {
	pypi := packagev1.Ecosystem_ECOSYSTEM_PYPI

	_, err := CompareVersions(pypi, "", "1.0")
	assert.Error(t, err)
	_, err = CompareVersions(pypi, "1.0", "")
	assert.Error(t, err)
}

// TestCompareVersionsIsTotal pins the OSV behaviour for a string that the
// registry would reject: it still gets an order, and the order is stable.
func TestCompareVersionsIsTotal(t *testing.T) {
	for _, bad := range []string{"not a version!", "1.0 beta", "0.1-bulbasaur"} {
		t.Run(bad, func(t *testing.T) {
			ab, err := CompareVersions(packagev1.Ecosystem_ECOSYSTEM_PYPI, "1.0", bad)
			require.NoError(t, err)
			ba, err := CompareVersions(packagev1.Ecosystem_ECOSYSTEM_PYPI, bad, "1.0")
			require.NoError(t, err)
			assert.Equal(t, -ab, ba)
		})
	}
}

func TestPackageVersionCompare(t *testing.T) {
	pypi := packagev1.Ecosystem_ECOSYSTEM_PYPI

	t.Run("orders two spellings of one package", func(t *testing.T) {
		a := NewPackageVersionFromParts(pypi, "Flask_RESTful", "0.3.10")
		b := NewPackageVersionFromParts(pypi, "flask.restful", "0.4.0rc1")
		got, err := a.Compare(b)
		require.NoError(t, err)
		assert.Equal(t, -1, got)
	})

	t.Run("one release in two spellings is equal", func(t *testing.T) {
		a := NewPackageVersionFromParts(pypi, "requests", "2.31")
		b := NewPackageVersionFromParts(pypi, "requests", "2.31.0")
		got, err := a.Compare(b)
		require.NoError(t, err)
		assert.Equal(t, 0, got)
	})

	t.Run("two packages have no order", func(t *testing.T) {
		goEco := packagev1.Ecosystem_ECOSYSTEM_GO
		a := NewPackageVersionFromParts(goEco, "github.com/Masterminds/goutils", "v1.0.0")
		b := NewPackageVersionFromParts(goEco, "github.com/masterminds/goutils", "v1.1.0")
		_, err := a.Compare(b)
		assert.ErrorIs(t, err, ErrDifferentPackages)
	})

	t.Run("two ecosystems have no order", func(t *testing.T) {
		a := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_NPM, "x", "1.0.0")
		b := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_CARGO, "x", "1.0.0")
		_, err := a.Compare(b)
		assert.ErrorIs(t, err, ErrDifferentPackages)
	})
}

// TestEveryEcosystemDecidesItsOrder fails when safedep/api adds an ecosystem
// and nobody decides whether its versions have an order.
func TestEveryEcosystemDecidesItsOrder(t *testing.T) {
	noOrder := map[packagev1.Ecosystem]bool{
		packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED:          true,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS:       true,
		packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY:    true,
		packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY:    true,
		packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY: true,
		packagev1.Ecosystem_ECOSYSTEM_HOMEBREW:             true,
	}
	for value := range packagev1.Ecosystem_name {
		ecosystem := packagev1.Ecosystem(value)
		assert.NotEqual(t, noOrder[ecosystem], HasVersionOrder(ecosystem),
			"%s must either have an order or be listed as having none", ecosystem)
	}
}

// TestVersionOrderConformance runs the OSV fixtures. The semver file covers
// npm, Cargo and Go, which share one order.
func TestVersionOrderConformance(t *testing.T) {
	ecosystems := map[string][]packagev1.Ecosystem{
		"pypi":      {packagev1.Ecosystem_ECOSYSTEM_PYPI},
		"maven":     {packagev1.Ecosystem_ECOSYSTEM_MAVEN},
		"nuget":     {packagev1.Ecosystem_ECOSYSTEM_NUGET},
		"packagist": {packagev1.Ecosystem_ECOSYSTEM_PACKAGIST},
		"rubygems":  {packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS},
		"pub":       {packagev1.Ecosystem_ECOSYSTEM_PUB},
		"semver": {
			packagev1.Ecosystem_ECOSYSTEM_NPM,
			packagev1.Ecosystem_ECOSYSTEM_CARGO,
			packagev1.Ecosystem_ECOSYSTEM_GO,
		},
	}

	files, err := filepath.Glob(filepath.Join("testdata", "version-order", "*.txt.gz"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	for _, file := range files {
		prefix, _, _ := strings.Cut(filepath.Base(file), "-")
		targets, ok := ecosystems[prefix]
		require.True(t, ok, "no ecosystem for %s", file)

		pairs := readVersionOrderFixture(t, file)
		require.NotEmpty(t, pairs, file)

		for _, ecosystem := range targets {
			t.Run(filepath.Base(file)+"/"+ecosystem.String(), func(t *testing.T) {
				for _, pair := range pairs {
					got, err := CompareVersions(ecosystem, pair.a, pair.b)
					if !assert.NoError(t, err, "%s %s %s", pair.a, pair.op, pair.b) {
						continue
					}
					assert.Equal(t, pair.want, got, "%s %s %s", pair.a, pair.op, pair.b)

					back, err := CompareVersions(ecosystem, pair.b, pair.a)
					require.NoError(t, err)
					assert.Equal(t, -pair.want, back, "%s %s %s, reversed", pair.a, pair.op, pair.b)
				}
			})
		}
	}
}

type versionOrderPair struct {
	a, op, b string
	want     int
}

func readVersionOrderFixture(t *testing.T, file string) []versionOrderPair {
	t.Helper()

	f, err := os.Open(file)
	require.NoError(t, err)
	defer func() { assert.NoError(t, f.Close()) }()

	gz, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer func() { assert.NoError(t, gz.Close()) }()

	ops := map[string]int{"<": -1, "=": 0, ">": 1}
	var pairs []versionOrderPair
	scanner := bufio.NewScanner(gz)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "// ") {
			continue
		}
		fields := strings.Split(line, " ")
		require.Len(t, fields, 3, "malformed line %q in %s", line, file)
		want, ok := ops[fields[1]]
		require.True(t, ok, "unknown operator in %q", line)
		pairs = append(pairs, versionOrderPair{a: fields[0], op: fields[1], b: fields[2], want: want})
	}
	require.NoError(t, scanner.Err())
	return pairs
}
