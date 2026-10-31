package pb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNameKeyPrefixSelects(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		purl    string
		selects bool
	}{
		{name: "npm scope", prefix: "pkg:npm/@acme/*", purl: "pkg:npm/@acme/lib@1.0.0", selects: true},
		{name: "npm scope encoded at", prefix: "pkg:npm/%40acme/*", purl: "pkg:npm/@acme/lib", selects: true},
		{name: "npm scope boundary", prefix: "pkg:npm/@acme/*", purl: "pkg:npm/@acmex/x"},
		{name: "npm scope is not a package", prefix: "pkg:npm/@acme/*", purl: "pkg:npm/acme"},
		{name: "npm is case-sensitive", prefix: "pkg:npm/@acme/*", purl: "pkg:npm/@ACME/lib"},
		{name: "npm dot", prefix: "pkg:npm/lodash.*", purl: "pkg:npm/lodash.merge", selects: true},
		{name: "npm dot is not the name", prefix: "pkg:npm/lodash.*", purl: "pkg:npm/lodash"},
		{name: "npm ecosystem", prefix: "pkg:npm/*", purl: "pkg:npm/anything", selects: true},
		{name: "npm ecosystem with scope", prefix: "pkg:npm/*", purl: "pkg:npm/@acme/lib", selects: true},
		{name: "npm ecosystem is not pypi", prefix: "pkg:npm/*", purl: "pkg:pypi/anything"},
		{name: "maven group dot", prefix: "pkg:maven/org.apache.*", purl: "pkg:maven/org.apache.commons/commons-lang3", selects: true},
		{name: "maven group dot boundary", prefix: "pkg:maven/org.apache.*", purl: "pkg:maven/org.apachex/x"},
		{name: "maven group dot is not the group", prefix: "pkg:maven/org.apache.*", purl: "pkg:maven/org.apache/x"},
		{name: "maven group", prefix: "pkg:maven/org.apache/*", purl: "pkg:maven/org.apache/x", selects: true},
		{name: "maven group is not a subgroup", prefix: "pkg:maven/org.apache/*", purl: "pkg:maven/org.apache.commons/x"},
		{name: "nuget dot", prefix: "pkg:nuget/Microsoft.*", purl: "pkg:nuget/Microsoft.Extensions.Logging", selects: true},
		{name: "nuget folds case", prefix: "pkg:nuget/Microsoft.*", purl: "pkg:nuget/microsoft.evil@1.0.0", selects: true},
		{name: "nuget dot boundary", prefix: "pkg:nuget/Microsoft.*", purl: "pkg:nuget/MicrosoftX.Y"},
		{name: "pypi folds dot to hyphen", prefix: "pkg:pypi/foo.*", purl: "pkg:pypi/foo-bar", selects: true},
		{name: "pypi folds dot to underscore", prefix: "pkg:pypi/foo.*", purl: "pkg:pypi/foo_bar", selects: true},
		{name: "pypi folds case", prefix: "pkg:pypi/Foo.*", purl: "pkg:pypi/FOO.Bar", selects: true},
		{name: "pypi boundary", prefix: "pkg:pypi/foo.*", purl: "pkg:pypi/foobar"},
		{name: "pypi is not the stem", prefix: "pkg:pypi/foo.*", purl: "pkg:pypi/foo"},
		{name: "cargo folds hyphen", prefix: "pkg:cargo/tokio.*", purl: "pkg:cargo/tokio.util", selects: true},
		{name: "go module path", prefix: "pkg:golang/github.com/acme/*", purl: "pkg:golang/github.com/acme/lib", selects: true},
		{name: "go module path boundary", prefix: "pkg:golang/github.com/acme/*", purl: "pkg:golang/github.com/acmex/lib"},
		{name: "composer vendor", prefix: "pkg:composer/Acme/*", purl: "pkg:composer/acme/lib", selects: true},
		{name: "github action owner", prefix: "pkg:githubactions/Actions/*", purl: "pkg:githubactions/actions/cache", selects: true},
		{name: "upper case type", prefix: "pkg:NPM/@acme/*", purl: "pkg:npm/@acme/lib", selects: true},
		{name: "encoded slash is not a boundary", prefix: "pkg:npm/@acme%2F.*", purl: "pkg:npm/@acme/lib"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prefix, err := NameKeyPrefix(tc.prefix)
			require.NoError(t, err)

			pv, err := NewPackageVersionFromPurl(tc.purl)
			require.NoError(t, err)

			assert.Equal(t, tc.selects, strings.HasPrefix(pv.NameKey(), prefix),
				"prefix %s, name key %s", prefix, pv.NameKey())
		})
	}
}

func TestNameKeyPrefixFails(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		wantErr string
	}{
		{name: "not a prefix", prefix: "pkg:npm/evil", wantErr: "does not end with"},
		{name: "no boundary", prefix: "pkg:npm/ev*", wantErr: `"/*" or ".*"`},
		{name: "encoded boundary", prefix: "pkg:npm/@acme%2F*", wantErr: `"/*" or ".*"`},
		{name: "second star", prefix: "pkg:npm/*/x/*", wantErr: "second"},
		{name: "qualifiers", prefix: "pkg:npm/x?a=b/*", wantErr: "qualifiers"},
		{name: "subpath", prefix: "pkg:npm/x#a/*", wantErr: "subpath"},
		{name: "encoded star", prefix: "pkg:npm/%2a/*", wantErr: "encoded"},
		{name: "no scheme", prefix: "npm/@acme/*", wantErr: "pkg:"},
		{name: "other scheme", prefix: "git:npm/@acme/*", wantErr: "pkg:"},
		{name: "no type", prefix: "pkg:npm.*", wantErr: "no type"},
		{name: "empty namespace segment", prefix: "pkg:npm/@acme//*", wantErr: "empty namespace"},
		{name: "version", prefix: "pkg:npm/x@1.*", wantErr: "version"},
		{name: "scoped version", prefix: "pkg:npm/@acme/x@1.*", wantErr: "version"},
		{name: "unknown type", prefix: "pkg:unknown/x.*", wantErr: "unsupported"},
		{name: "namespace in pypi", prefix: "pkg:pypi/acme/*", wantErr: "no namespace"},
		{name: "bad escape", prefix: "pkg:npm/%zz.*", wantErr: "invalid"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NameKeyPrefix(tc.prefix)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestNameKeyPrefixScopedNpmAt(t *testing.T) {
	// An "@" that follows "/" starts an npm scope, not a version.
	prefix, err := NameKeyPrefix("pkg:npm/@acme.*")
	require.NoError(t, err)

	pv, err := NewPackageVersionFromPurl("pkg:npm/@acme.tools/lib")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(pv.NameKey(), prefix))
}

func TestNameKeyPrefixNuGetNonASCII(t *testing.T) {
	prefix, err := NameKeyPrefix("pkg:nuget/Microsoft.*")
	require.NoError(t, err)

	pv, err := NewPackageVersionFromPurl("pkg:nuget/Microsoft.%C3%89vil")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(pv.NameKey(), prefix), "%q does not start with %q", pv.NameKey(), prefix)
}

// FuzzNameKeyPrefix checks that NameKeyPrefix never panics, and that a
// package whose ASCII name continues the prefix has a NameKey that starts
// with the result.
//
// The property does not hold for a non-ASCII name under an ecosystem whose
// fold keeps such a name raw, as "pkg:pypi/foo.*" and "foo.\u00e9". Those
// registries accept only ASCII names. NuGet accepts non-ASCII ids, and
// TestNameKeyPrefixNuGetNonASCII covers it.
func FuzzNameKeyPrefix(f *testing.F) {
	for _, seed := range []string{
		"pkg:npm/@acme/*",
		"pkg:npm/*",
		"pkg:maven/org.apache.*",
		"pkg:maven/org.apache/*",
		"pkg:pypi/Foo.*",
		"pkg:nuget/Microsoft.*",
		"pkg:githubactions/Actions/Cache/*",
		"pkg:golang/github.com/acme/*",
		"pkg:composer/Acme/*",
		"pkg:npm/%40acme/*",
		"pkg:npm/x@1.*",
		"pkg:pypi/acme/*",
	} {
		f.Add(seed, "lib")
	}

	f.Fuzz(func(t *testing.T, purl, rest string) {
		prefix, err := NameKeyPrefix(purl)
		if err != nil {
			return
		}

		// A rest with a separator, an escape or a version marker would
		// change how the PURL splits, so the property holds only for a plain
		// rest.
		if rest == "" || strings.ContainsAny(rest, "/@?#%*") || !isASCII(purl+rest) {
			return
		}

		pv, err := NewPackageVersionFromPurl(strings.TrimSuffix(purl, "*") + rest)
		if err != nil || pv.RawVersion() != "" {
			return
		}

		assert.True(t, strings.HasPrefix(pv.NameKey(), prefix),
			"purl %q, rest %q: name key %q does not start with %q", purl, rest, pv.NameKey(), prefix)
	})
}
