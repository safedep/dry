package pb

import (
	"reflect"
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
		{packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, 0},
		{packagev1.Ecosystem_ECOSYSTEM_CARGO, 0},
		{packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, 0},
		{packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, 0},
		{packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY, 0},
		{packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY, 0},
		{packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY, 0},
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
		fromParts := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, "vendor-a/library", "1.0.0")

		assert.Equal(t, "vendor-a/library", vendorA.Name())
		assert.False(t, vendorA.Equal(vendorB), "two vendors are two packages")
		assert.NotEqual(t, vendorA.Key(), vendorB.Key())
		assert.True(t, vendorA.Equal(fromParts), "both constructors must agree")

		urn, err := fromParts.URN()
		require.NoError(t, err)
		assert.Equal(t, "pkg:composer/vendor-a/library@1.0.0", urn)
	})

	t.Run("malformed purl is an error", func(t *testing.T) {
		_, err := NewPackageVersionFromPurl("not a purl")
		require.Error(t, err)
	})

	t.Run("purl type outside the ecosystem enum is an error", func(t *testing.T) {
		// deb and rpm would both collapse to an unspecified ecosystem with
		// the bare name curl, so two distinct packages would share one key.
		for _, purl := range []string{"pkg:deb/debian/curl@1.0", "pkg:rpm/fedora/curl@1.0"} {
			_, err := NewPackageVersionFromPurl(purl)
			assert.ErrorContains(t, err, "unsupported purl type", purl)
		}
	})

	t.Run("urn fails for an ecosystem with no purl type", func(t *testing.T) {
		pv := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, "x", "1")
		_, err := pv.URN()
		require.Error(t, err)
	})
}

// TestNewPackageVersionFromPurlAgreesWithParts pins the extraction boundary:
// the purl constructor keeps the coordinates as written, then folds them
// under the same rule as the parts constructor, so both name one identity
// and URN() round-trips to it. Every prefix, alias, encoding and namespace
// spelling the parser accepts must land on the same value.
func TestNewPackageVersionFromPurlAgreesWithParts(t *testing.T) {
	cases := []struct {
		name      string
		purl      string
		ecosystem packagev1.Ecosystem
		rawName   string
		version   string
		wantName  string
	}{
		{"go keeps case", "pkg:golang/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go upper case scheme", "PKG:golang/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go one slash after scheme", "pkg:/golang/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go two slashes after scheme", "pkg://golang/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go three slashes after scheme", "pkg:///golang/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go type alias", "pkg:go/example.com/Owner/Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go empty namespace segment", "pkg:golang/example.com/Owner//Library@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go percent-encoded slash", "pkg:golang/example.com/Owner%2FLibrary@v1.0.0", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"go percent-encoded version", "pkg:golang/example.com/Owner/Library@v1.0.0%2Bincompatible", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0+incompatible", "example.com/Owner/Library"},
		{"go qualifiers and subpath", "pkg:golang/example.com/Owner/Library@v1.0.0?type=module#cmd/tool", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0", "example.com/Owner/Library"},
		{"pypi keeps raw spelling", "pkg:pypi/Flask_RESTful@1.0", packagev1.Ecosystem_ECOSYSTEM_PYPI, "Flask_RESTful", "1.0", "flask-restful"},
		{"pypi type alias", "pkg:pip/Flask_RESTful@1.0", packagev1.Ecosystem_ECOSYSTEM_PYPI, "Flask_RESTful", "1.0", "flask-restful"},
		{"npm keeps case", "pkg:npm/JSONStream@1.0.3", packagev1.Ecosystem_ECOSYSTEM_NPM, "JSONStream", "1.0.3", "JSONStream"},
		{"npm scope keeps case", "pkg:npm/@Vue/Reactivity@3.0.0", packagev1.Ecosystem_ECOSYSTEM_NPM, "@Vue/Reactivity", "3.0.0", "@Vue/Reactivity"},
		{"npm percent-encoded scope", "pkg:npm/%40Vue/Reactivity@3.0.0", packagev1.Ecosystem_ECOSYSTEM_NPM, "@Vue/Reactivity", "3.0.0", "@Vue/Reactivity"},
		{"composer keeps case until its rule ships", "pkg:composer/Vendor-A/Library@1.0.0", packagev1.Ecosystem_ECOSYSTEM_PACKAGIST, "Vendor-A/Library", "1.0.0", "Vendor-A/Library"},
		{"github keeps case until its rule ships", "pkg:github/Owner/Library@main", packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, "Owner/Library", "main", "Owner/Library"},
		{"github type alias", "pkg:actions/Owner/Library@main", packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS, "Owner/Library", "main", "Owner/Library"},
		{"bitbucket keeps case until its rule ships", "pkg:bitbucket/Owner/Library@244fd47", packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY, "Owner/Library", "244fd47", "Owner/Library"},
		{"gitlab keeps case", "pkg:gitlab/Group/Project@1.2", packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY, "Group/Project", "1.2", "Group/Project"},
		{"maven keeps case", "pkg:maven/com.google.Guava/guava@32.0", packagev1.Ecosystem_ECOSYSTEM_MAVEN, "com.google.Guava:guava", "32.0", "com.google.Guava:guava"},
		{"rubygems type alias", "pkg:rubygems/Nokogiri@1.16.0", packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS, "Nokogiri", "1.16.0", "Nokogiri"},
		{"cargo keeps case until its rule ships", "pkg:cargo/Serde_JSON@1.0.0", packagev1.Ecosystem_ECOSYSTEM_CARGO, "Serde_JSON", "1.0.0", "Serde_JSON"},
		{"nuget keeps case", "pkg:nuget/Newtonsoft.Json@13.0.1", packagev1.Ecosystem_ECOSYSTEM_NUGET, "Newtonsoft.Json", "13.0.1", "Newtonsoft.Json"},
		{"no version", "pkg:golang/example.com/Owner/Library", packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "", "example.com/Owner/Library"},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fromPurl, err := NewPackageVersionFromPurl(test.purl)
			require.NoError(t, err)
			fromParts := NewPackageVersionFromParts(test.ecosystem, test.rawName, test.version)

			assert.Equal(t, test.ecosystem, fromPurl.Ecosystem())
			assert.Equal(t, test.rawName, fromPurl.RawName(), "the raw name is the spelling in the purl")
			assert.Equal(t, test.version, fromPurl.RawVersion(), "the raw version is the spelling in the purl")
			assert.Equal(t, test.wantName, fromPurl.Name())
			assert.True(t, fromPurl.Equal(fromParts), "both constructors must agree")
			assert.Equal(t, fromParts.Key(), fromPurl.Key())

			urn, err := fromPurl.URN()
			require.NoError(t, err)
			again, err := NewPackageVersionFromPurl(urn)
			require.NoError(t, err)
			assert.True(t, fromPurl.Equal(again), "URN() must round-trip to the same identity: %s", urn)
		})
	}
}

// TestNewPackageVersionFromPurlCase pins which purl types keep case as an
// identity and which fold it, so a change in either direction is a visible
// change to a published rule. Only PyPI folds until the other rules ship with
// their fixtures, even where the frozen helper lower-cases the name.
func TestNewPackageVersionFromPurlCase(t *testing.T) {
	cases := []struct {
		name  string
		upper string
		lower string
		same  bool
	}{
		{"go module path is case-sensitive", "pkg:golang/example.com/Owner/Library@v1.0.0", "pkg:golang/example.com/owner/library@v1.0.0", false},
		{"npm name is case-sensitive", "pkg:npm/JSONStream@1.0.3", "pkg:npm/jsonstream@1.0.3", false},
		{"gitlab path is case-sensitive", "pkg:gitlab/Group/Project@1.2", "pkg:gitlab/group/project@1.2", false},
		{"github keeps case until its rule ships", "pkg:github/Owner/Library@main", "pkg:github/owner/library@main", false},
		{"bitbucket keeps case until its rule ships", "pkg:bitbucket/Owner/Library@244fd47", "pkg:bitbucket/owner/library@244fd47", false},
		{"composer keeps case until its rule ships", "pkg:composer/Vendor-A/Library@1.0.0", "pkg:composer/vendor-a/library@1.0.0", false},
		{"pypi name folds", "pkg:pypi/Flask_RESTful@1.0", "pkg:pypi/flask-restful@1.0", true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			upper, err := NewPackageVersionFromPurl(test.upper)
			require.NoError(t, err)
			lower, err := NewPackageVersionFromPurl(test.lower)
			require.NoError(t, err)

			assert.Equal(t, test.same, upper.Equal(lower))
			assert.Equal(t, test.same, upper.Key() == lower.Key())
		})
	}
}

func TestPackageVersionProtoIsFresh(t *testing.T) {
	pv := NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "CalcBoxLite", "1.0.0")

	canonical := pv.CanonicalProto()
	canonical.Package.Name = "mutated"
	canonical.Version = "mutated"

	raw := pv.RawProto()
	raw.Package.Name = "mutated"
	raw.Version = "mutated"

	assert.Equal(t, "calcboxlite", pv.Name())
	assert.Equal(t, "1", pv.Version())
	assert.Equal(t, "CalcBoxLite", pv.RawName())
	assert.Equal(t, "1.0.0", pv.RawVersion())

	again := pv.CanonicalProto()
	assert.Equal(t, "calcboxlite", again.GetPackage().GetName())
	assert.Equal(t, "1", again.GetVersion())
	assert.NotSame(t, canonical, again)

	rawAgain := pv.RawProto()
	assert.Equal(t, "CalcBoxLite", rawAgain.GetPackage().GetName())
	assert.Equal(t, "1.0.0", rawAgain.GetVersion())
}

func TestPackageVersionIsNotComparable(t *testing.T) {
	// A comparable value would let a == b and a map keyed on the value
	// compare the raw spelling, and miss an identity written under another
	// spelling. Equal and Key are the two supported comparisons.
	assert.False(t, reflect.TypeFor[PackageVersion]().Comparable())
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

}

// TestPackageVersionKeyFormat pins the key byte for byte, because a
// persistent cache stores it. The ecosystem part is the proto enum value
// name. buf breaking runs on safedep/api with ENUM_VALUE_SAME_NAME, so a
// rename of a value is a rejected change there, not a silent key change here.
func TestPackageVersionKeyFormat(t *testing.T) {
	npm := packagev1.Ecosystem_ECOSYSTEM_NPM
	cases := []struct {
		pv   PackageVersion
		want string
	}{
		{NewPackageVersionFromParts(npm, "@scope/pkg", "1.0.0+build"), "ECOSYSTEM_NPM/0/%40scope%2Fpkg@1.0.0%2Bbuild"},
		{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "CalcBoxLite", "1.0.0"), "ECOSYSTEM_PYPI/1/calcboxlite@1"},
		{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_CARGO, "Serde_JSON", "1.0.0+build"), "ECOSYSTEM_CARGO/0/Serde_JSON@1.0.0%2Bbuild"},
		{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_GO, "example.com/Owner/Library", "v1.0.0"), "ECOSYSTEM_GO/0/example.com%2FOwner%2FLibrary@v1.0.0"},
		{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_PYPI, "", ""), "ECOSYSTEM_PYPI/1/@"},
		{NewPackageVersionFromParts(packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED, "x", "1"), "ECOSYSTEM_UNSPECIFIED/0/x@1"},
	}
	for _, test := range cases {
		assert.Equal(t, test.want, test.pv.Key())
	}
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
