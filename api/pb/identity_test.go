package pb

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentityRuleVersion(t *testing.T) {
	cases := []struct {
		ecosystem packagev1.Ecosystem
		want      int
	}{
		{packagev1.Ecosystem_ECOSYSTEM_PYPI, 1},
		{packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, 1},
		{packagev1.Ecosystem_ECOSYSTEM_CARGO, 1},
		{packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, 1},
		{packagev1.Ecosystem_ECOSYSTEM_NPM, 0},
		{packagev1.Ecosystem_ECOSYSTEM_GO, 0},
		{packagev1.Ecosystem_ECOSYSTEM_MAVEN, 0},
		{packagev1.Ecosystem_ECOSYSTEM_NUGET, 0},
		{packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, 0},
	}

	for _, test := range cases {
		t.Run(test.ecosystem.String(), func(t *testing.T) {
			assert.Equal(t, test.want, IdentityRuleVersion(test.ecosystem))
		})
	}
}

func TestNewPackageVersionFromProto(t *testing.T) {
	t.Run("nil is total", func(t *testing.T) {
		pv := NewPackageVersion(nil)
		assert.Equal(t, packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, pv.Ecosystem())
		assert.Empty(t, pv.Name())
		assert.Empty(t, pv.Version())
		assert.False(t, pv.HasRule())
	})

	t.Run("proto folds like parts", func(t *testing.T) {
		proto := newPackageVersionProto(packagev1.Ecosystem_ECOSYSTEM_PYPI, "Flask_RESTful", "0!0.3.10.0")
		fromProto := NewPackageVersion(proto)
		fromParts := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "Flask_RESTful", "0!0.3.10.0")
		assert.True(t, fromProto.Equal(fromParts))
		assert.Equal(t, "flask-restful", fromProto.Name())
		assert.Equal(t, "0.3.10", fromProto.Version())
	})
}

func TestNewPackageVersionFromPurl(t *testing.T) {
	t.Run("pypi", func(t *testing.T) {
		pv, err := NewPackageVersionFromPurl("pkg:pypi/Flask_RESTful@0.3.10.0")
		require.NoError(t, err)
		assert.Equal(t, "flask-restful", pv.Name())
		assert.Equal(t, "0.3.10", pv.Version())
		assert.Equal(t, "0.3.10.0", pv.RawVersion())

		urn, err := pv.URN()
		require.NoError(t, err)
		assert.Equal(t, "pkg:pypi/flask-restful@0.3.10", urn)
	})

	t.Run("npm keeps case through the purl parser", func(t *testing.T) {
		pv, err := NewPackageVersionFromPurl("pkg:npm/JSONStream@1.0.3")
		require.NoError(t, err)
		assert.Equal(t, "JSONStream", pv.Name())
	})

	t.Run("go module path keeps case through the purl parser", func(t *testing.T) {
		fromPurl, err := NewPackageVersionFromPurl("pkg:golang/example.com/Owner/Library@v1.0.0")
		require.NoError(t, err)
		fromParts := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0")

		assert.Equal(t, "example.com/Owner/Library", fromPurl.Name())
		assert.True(t, fromPurl.Equal(fromParts), "both constructors must agree")

		lower, err := NewPackageVersionFromPurl("pkg:golang/example.com/owner/library@v1.0.0")
		require.NoError(t, err)
		assert.False(t, fromPurl.Equal(lower), "a Go module path is case-sensitive")
		assert.NotEqual(t, fromPurl.Key(), lower.Key())

		urn, err := fromPurl.URN()
		require.NoError(t, err)
		assert.Equal(t, "pkg:golang/example.com/Owner/Library@v1.0.0", urn)
	})

	t.Run("go purl with qualifiers and subpath", func(t *testing.T) {
		pv, err := NewPackageVersionFromPurl("pkg:golang/github.com/safedep/Vet@v1.0.0?type=module#cmd/vet")
		require.NoError(t, err)
		assert.Equal(t, "github.com/safedep/Vet", pv.Name())
		assert.Equal(t, "v1.0.0", pv.Version())
	})

	t.Run("composer keeps its vendor", func(t *testing.T) {
		vendorA, err := NewPackageVersionFromPurl("pkg:composer/vendor-a/library@1.0.0")
		require.NoError(t, err)
		vendorB, err := NewPackageVersionFromPurl("pkg:composer/vendor-b/library@1.0.0")
		require.NoError(t, err)
		fromParts := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, "Vendor-A/Library", "1.0.0")

		assert.Equal(t, "vendor-a/library", vendorA.Name())
		assert.False(t, vendorA.Equal(vendorB), "two vendors are two packages")
		assert.NotEqual(t, vendorA.Key(), vendorB.Key())
		assert.True(t, vendorA.Equal(fromParts), "both constructors must agree")

		urn, err := fromParts.URN()
		require.NoError(t, err)
		assert.Equal(t, "pkg:composer/vendor-a/library@1.0.0", urn)
	})

	t.Run("malformed purl is the one error", func(t *testing.T) {
		_, err := NewPackageVersionFromPurl("not a purl")
		require.Error(t, err)
	})

	t.Run("urn fails for an ecosystem with no purl type", func(t *testing.T) {
		pv := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, "x", "1")
		_, err := pv.URN()
		require.Error(t, err)
	})
}

func TestPackageVersionProtoIsFresh(t *testing.T) {
	pv := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "CalcBoxLite", "1.0.0")

	canonical := pv.Proto()
	canonical.Package.Name = "mutated"
	canonical.Version = "mutated"

	raw := pv.RawProto()
	raw.Package.Name = "mutated"
	raw.Version = "mutated"

	assert.Equal(t, "calcboxlite", pv.Name())
	assert.Equal(t, "1", pv.Version())
	assert.Equal(t, "CalcBoxLite", pv.RawName())
	assert.Equal(t, "1.0.0", pv.RawVersion())

	again := pv.Proto()
	assert.Equal(t, "calcboxlite", again.GetPackage().GetName())
	assert.Equal(t, "1", again.GetVersion())
	assert.NotSame(t, canonical, again)

	rawAgain := pv.RawProto()
	assert.Equal(t, "CalcBoxLite", rawAgain.GetPackage().GetName())
	assert.Equal(t, "1.0.0", rawAgain.GetVersion())
}

func TestPackageVersionEqual(t *testing.T) {
	pypi := packagev1.Ecosystem_ECOSYSTEM_PYPI

	a := NewPackageVersionFromParts(pypi, "CalcBoxLite", "1.0")
	b := NewPackageVersionFromParts(pypi, "calcboxlite", "1.0.0")
	c := NewPackageVersionFromParts(pypi, "calc-box-lite", "1.0")
	d := NewPackageVersionFromParts(pypi, "calcboxlite", "1.0rc1")
	npm := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_NPM, "calcboxlite", "1")

	assert.True(t, a.Equal(b))
	assert.Equal(t, a.Key(), b.Key())
	assert.False(t, a.Equal(c), "a different package on PyPI")
	assert.False(t, a.Equal(d), "a different release on PyPI")
	assert.False(t, a.Equal(npm), "a different ecosystem")
}

func TestPackageVersionKeyDoesNotAlias(t *testing.T) {
	// The constructor is total, so a name or a version can hold the
	// separators the key uses. Two distinct identities must never share a key.
	npm := packagev1.Ecosystem_ECOSYSTEM_NPM

	cases := []struct {
		name string
		a    PackageVersion
		b    PackageVersion
	}{
		{
			name: "at sign moves between name and version",
			a:    NewPackageVersionFromParts(npm, "a", "b@c"),
			b:    NewPackageVersionFromParts(npm, "a@b", "c"),
		},
		{
			name: "slash moves between name and version",
			a:    NewPackageVersionFromParts(npm, "@scope/a", "1"),
			b:    NewPackageVersionFromParts(npm, "@scope", "a@1"),
		},
		{
			name: "scoped npm name keeps one key",
			a:    NewPackageVersionFromParts(npm, "@scope/pkg", "1.0.0"),
			b:    NewPackageVersionFromParts(npm, "@scope/pkg", "1.0.0"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.a.Equal(test.b), test.a.Key() == test.b.Key())
		})
	}

	t.Run("format is stable", func(t *testing.T) {
		cases := []struct {
			pv   PackageVersion
			want string
		}{
			{NewPackageVersionFromParts(npm, "@scope/pkg", "1.0.0+build"), "ECOSYSTEM_NPM/0/%40scope%2Fpkg@1.0.0%2Bbuild"},
			{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "CalcBoxLite", "1.0.0"), "ECOSYSTEM_PYPI/1/calcboxlite@1"},
			{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_CARGO, "Serde_JSON", "1.0.0+build"), "ECOSYSTEM_CARGO/1/serde_json@1.0.0%2Bbuild"},
			{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "", ""), "ECOSYSTEM_PYPI/1/@"},
		}
		for _, test := range cases {
			assert.Equal(t, test.want, test.pv.Key())
		}
	})
}

func TestPep440Version(t *testing.T) {
	cases := []struct {
		input string
		want  string
		ok    bool
	}{
		{"1.0", "1", true},
		{"1.0.0", "1", true},
		{"0.0.0", "0", true},
		{"0!1.0", "1", true},
		{"01!002.00", "1!2", true},
		{"1.0.0rc1", "1rc1", true},
		{"V01.0RC01.POST02.DEV03+LOCAL_004-ABC", "1rc1.post2.dev3+local.4.abc", true},
		{"1.0.1", "1.0.1", true},
		{"1.0-1", "1.post1", true},
		{"1.0_1", "1.0_1", false},
		{"latest", "latest", false},
		{"", "", false},
		// The Kelvin sign case-folds to k. PEP 440 is ASCII, so it stays raw.
		{"1+\u212a", "1+\u212a", false},
		// A no-break space is whitespace to packaging and to strings.TrimSpace.
		{"1.0\u00a0", "1", true},
		// The information separators are whitespace to Python but not to Go.
		{"\x1c1.0\x1d", "1", true},
		{"\x1e1.0\x1f", "1", true},
	}

	for _, test := range cases {
		t.Run(test.input, func(t *testing.T) {
			got, ok := pep440Version(test.input)
			assert.Equal(t, test.ok, ok)
			assert.Equal(t, test.want, got)

			again, _ := pep440Version(got)
			assert.Equal(t, got, again, "the fold must be idempotent")
		})
	}
}

func TestIdentityRulesTable(t *testing.T) {
	// A rule in the table is a published rule: it has a version and a name
	// fold, and every fold is idempotent, because a second write of one
	// package must land on the row the first created.
	for ecosystem, rule := range identityRules {
		t.Run(ecosystem.String(), func(t *testing.T) {
			assert.Positive(t, rule.version)
			require.NotNil(t, rule.foldName)

			once := rule.foldName("Some_Name.Here")
			assert.Equal(t, once, rule.foldName(once))

			if rule.foldVersion == nil {
				return
			}
			canonical, parsed := rule.foldVersion("1.0.0")
			require.True(t, parsed)
			again, _ := rule.foldVersion(canonical)
			assert.Equal(t, canonical, again)
		})
	}

	t.Run("absent ecosystem has the identity rule", func(t *testing.T) {
		rule := ruleFor(packagev1.Ecosystem_ECOSYSTEM_NPM)
		assert.False(t, rule.hasRule())
		assert.False(t, rule.hasVersionRule())
		assert.Equal(t, "JSONStream", rule.foldName("JSONStream"))
	})
}
