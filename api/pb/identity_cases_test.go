package pb

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

// identityCase is one row of the canonicalization catalog: what a producer
// observed, and what the fold makes of it. The catalog is the one place to
// read every rule and every corner case across ecosystems. Add a row here
// for every new case, and add a matching row to the ecosystem's fixture under
// testdata/identity when the reference implementation can generate it.
type identityCase struct {
	name        string
	ecosystem   packagev1.Ecosystem
	rawName     string
	rawVersion  string
	wantName    string
	wantVersion string
	wantParsed  bool
}

var (
	pypi      = packagev1.Ecosystem_ECOSYSTEM_PYPI
	npm       = packagev1.Ecosystem_ECOSYSTEM_NPM
	rubygems  = packagev1.Ecosystem_ECOSYSTEM_RUBYGEMS
	cargo     = packagev1.Ecosystem_ECOSYSTEM_CARGO
	packagist = packagev1.Ecosystem_ECOSYSTEM_PACKAGIST
	github    = packagev1.Ecosystem_ECOSYSTEM_GITHUB_ACTIONS
	githubRep = packagev1.Ecosystem_ECOSYSTEM_GITHUB_REPOSITORY
	bitbucket = packagev1.Ecosystem_ECOSYSTEM_BITBUCKET_REPOSITORY
	gitlab    = packagev1.Ecosystem_ECOSYSTEM_GITLAB_REPOSITORY
	golang    = packagev1.Ecosystem_ECOSYSTEM_GO
	maven     = packagev1.Ecosystem_ECOSYSTEM_MAVEN
	nuget     = packagev1.Ecosystem_ECOSYSTEM_NUGET
	unknown   = packagev1.Ecosystem_ECOSYSTEM_UNSPECIFIED
)

var identityCases = []identityCase{
	// PyPI names: PEP 503. Lower case, every run of - _ . folds to one hyphen.
	{"pypi name upper case", pypi, "DataToolKit", "1", "datatoolkit", "1", true},
	{"pypi name underscore", pypi, "Flask_RESTful", "1", "flask-restful", "1", true},
	{"pypi name dot", pypi, "zope.interface", "1", "zope-interface", "1", true},
	{"pypi name separator run", pypi, "Zope._-.Interface", "1", "zope-interface", "1", true},
	{"pypi name with separators is another package", pypi, "Data_Tool.Kit", "1", "data-tool-kit", "1", true},
	{"pypi name trailing separator", pypi, "name.", "1", "name-", "1", true},
	{"pypi name non-ascii stays raw", pypi, "\u0130nvoke", "1", "\u0130nvoke", "1", true},
	{"pypi name kelvin sign stays raw", pypi, "\u212aeras", "1", "\u212aeras", "1", true},
	{"pypi name invalid utf-8 stays raw", pypi, "\x80Name", "1", "\x80Name", "1", true},

	// PyPI versions: PEP 440 canonical form, as packaging.utils.canonicalize_version.
	{"pypi version release", pypi, "pkg", "1.0", "pkg", "1", true},
	{"pypi version trailing zeros", pypi, "pkg", "1.0.0.0", "pkg", "1", true},
	{"pypi version keeps inner zero", pypi, "pkg", "1.0.1", "pkg", "1.0.1", true},
	{"pypi version zero release", pypi, "pkg", "0.0.0", "pkg", "0", true},
	{"pypi version leading zeros", pypi, "pkg", "01.0", "pkg", "1", true},
	{"pypi version v prefix", pypi, "pkg", "v1.0", "pkg", "1", true},
	{"pypi version upper v prefix", pypi, "pkg", "V1.0", "pkg", "1", true},
	{"pypi version zero epoch", pypi, "pkg", "0!1.0", "pkg", "1", true},
	{"pypi version padded zero epoch", pypi, "pkg", "000!1.0", "pkg", "1", true},
	{"pypi version epoch kept", pypi, "pkg", "01!002.00", "pkg", "1!2", true},
	{"pypi version pre release trailing zero", pypi, "pkg", "1.0.0rc1", "pkg", "1rc1", true},
	{"pypi version pre release case", pypi, "pkg", "1.0RC1", "pkg", "1rc1", true},
	{"pypi version alpha spelling", pypi, "pkg", "1.0alpha", "pkg", "1a0", true},
	{"pypi version beta spelling with separators", pypi, "pkg", "1.0_beta_02", "pkg", "1b2", true},
	{"pypi version c spelling", pypi, "pkg", "1.0c1", "pkg", "1rc1", true},
	{"pypi version preview spelling", pypi, "pkg", "1.0preview1", "pkg", "1rc1", true},
	{"pypi version pre spelling", pypi, "pkg", "1.0pre1", "pkg", "1rc1", true},
	{"pypi version implicit post release", pypi, "pkg", "1.0-1", "pkg", "1.post1", true},
	{"pypi version padded implicit post release", pypi, "pkg", "1.0-01", "pkg", "1.post1", true},
	{"pypi version r spelling", pypi, "pkg", "1.0R02", "pkg", "1.post2", true},
	{"pypi version rev spelling", pypi, "pkg", "1.0rev", "pkg", "1.post0", true},
	{"pypi version bare post", pypi, "pkg", "1.0.post", "pkg", "1.post0", true},
	{"pypi version dev case", pypi, "pkg", "1.0DEV", "pkg", "1.dev0", true},
	{"pypi version dev number", pypi, "pkg", "1.0-dev1", "pkg", "1.dev1", true},
	{"pypi version local separators", pypi, "pkg", "1.0+ubuntu-1", "pkg", "1+ubuntu.1", true},
	{"pypi version local leading zeros", pypi, "pkg", "1.0+abc.007", "pkg", "1+abc.7", true},
	{
		"pypi version every segment",
		pypi,
		"pkg",
		"V01.0RC01.POST02.DEV03+LOCAL_004-ABC",
		"pkg",
		"1rc1.post2.dev3+local.4.abc",
		true,
	},
	{"pypi version surrounding whitespace", pypi, "pkg", " 1.0\n", "pkg", "1", true},
	{"pypi version no-break space is whitespace", pypi, "pkg", "1.0\u00a0", "pkg", "1", true},
	{"pypi version information separators are whitespace to python", pypi, "pkg", "\x1c1.0\x1f", "pkg", "1", true},
	{"pypi version line separator is whitespace", pypi, "pkg", "\u20281.0", "pkg", "1", true},
	{
		"pypi version huge epoch",
		pypi,
		"pkg",
		"999999999999999999999999999!1.0",
		"pkg",
		"999999999999999999999999999!1",
		true,
	},
	{"pypi version long numeric part", pypi, "pkg", "1.000000000000000000001", "pkg", "1.1", true},

	// PyPI versions the grammar rejects stay raw. They match only themselves.
	{"pypi version underscore post is invalid", pypi, "pkg", "1.0_1", "pkg", "1.0_1", false},
	{"pypi version final is invalid", pypi, "pkg", "1.0final", "pkg", "1.0final", false},
	{"pypi version two pre segments", pypi, "pkg", "1.0rc1rc2", "pkg", "1.0rc1rc2", false},
	{"pypi version post after dev", pypi, "pkg", "1.0.dev1.post1", "pkg", "1.0.dev1.post1", false},
	{"pypi version two epochs", pypi, "pkg", "1!2!1.0", "pkg", "1!2!1.0", false},
	{"pypi version empty local segment", pypi, "pkg", "1.0+local..1", "pkg", "1.0+local..1", false},
	{"pypi version dangling plus", pypi, "pkg", "1.0+", "pkg", "1.0+", false},
	{
		"pypi version wheel tags are not a version",
		pypi,
		"pkg",
		"1.0.0-py3-none-any",
		"pkg",
		"1.0.0-py3-none-any",
		false,
	},
	{"pypi version word", pypi, "pkg", "latest", "pkg", "latest", false},
	{"pypi version empty", pypi, "pkg", "", "pkg", "", false},
	{"pypi version kelvin sign is not k", pypi, "pkg", "1+\u212a", "pkg", "1+\u212a", false},
	{"pypi version non-ascii digit", pypi, "pkg", "\u0661.0", "pkg", "\u0661.0", false},
	{"pypi version byte order mark is not whitespace", pypi, "pkg", "\ufeff1.0", "pkg", "\ufeff1.0", false},
	{"pypi version zero width space is not whitespace", pypi, "pkg", "\u200b1.0", "pkg", "\u200b1.0", false},

	// npm: no rule. The registry is case-sensitive, so JSONStream and
	// jsonstream are two packages, and versions are semver as published.
	{"npm name keeps case", npm, "JSONStream", "1.0.3", "JSONStream", "1.0.3", false},
	{"npm name lower case is another package", npm, "jsonstream", "1.0.3", "jsonstream", "1.0.3", false},
	{"npm scoped name keeps case", npm, "@Vue/Reactivity", "3.0.0", "@Vue/Reactivity", "3.0.0", false},
	{"npm name keeps dot", npm, "socket.io", "4.0.0", "socket.io", "4.0.0", false},
	{"npm version keeps v prefix", npm, "express", "v4.17.1", "express", "v4.17.1", false},
	{"npm version keeps build metadata", npm, "express", "4.17.1+build", "express", "4.17.1+build", false},

	// No rule yet: RubyGems, Cargo, Packagist, GitHub and Bitbucket keep the
	// raw name and the raw version until each ships its rule with a fixture.
	// The frozen CanonicalPackageName lower-cases some of these. The type does
	// not, and its callers move to it under the spec's rollout.
	{"rubygems name keeps case", rubygems, "Nokogiri", "1.16.0", "Nokogiri", "1.16.0", false},
	{"rubygems version keeps trailing zero", rubygems, "rails", "7.0", "rails", "7.0", false},
	{"cargo name keeps case and underscore", cargo, "Serde_JSON", "1.0.0", "Serde_JSON", "1.0.0", false},
	{"cargo name keeps hyphen", cargo, "tokio-util", "0.7.0", "tokio-util", "0.7.0", false},
	{"cargo version keeps build metadata", cargo, "serde", "1.0.0+build", "serde", "1.0.0+build", false},
	{"packagist name keeps case", packagist, "Monolog/Monolog", "3.0.0", "Monolog/Monolog", "3.0.0", false},
	{"packagist version keeps short form", packagist, "monolog/monolog", "3.0", "monolog/monolog", "3.0", false},
	{"packagist name keeps its vendor", packagist, "vendor-a/library", "1.0.0", "vendor-a/library", "1.0.0", false},
	{"github action keeps case", github, "Actions/Checkout", "v4", "Actions/Checkout", "v4", false},
	{"github action keeps ref case", github, "actions/checkout", "V4", "actions/checkout", "V4", false},
	{"github repository keeps case", githubRep, "SafeDep/Vet", "main", "SafeDep/Vet", "main", false},
	{
		"bitbucket repository keeps case",
		bitbucket,
		"Birkenfeld/Pygments-Main",
		"244fd47",
		"Birkenfeld/Pygments-Main",
		"244fd47",
		false,
	},

	// No rule: raw name and raw version, case and all.
	{"gitlab path keeps case", gitlab, "Inkscape/Inkscape", "1.2", "Inkscape/Inkscape", "1.2", false},
	{
		"go module path keeps case",
		golang,
		"github.com/safedep/Vet",
		"v1.0.0",
		"github.com/safedep/Vet",
		"v1.0.0",
		false,
	},
	{"maven coordinate keeps case", maven, "com.google.Guava:guava", "32.0", "com.google.Guava:guava", "32.0", false},
	{"nuget id keeps case", nuget, "Newtonsoft.Json", "13.0.1", "Newtonsoft.Json", "13.0.1", false},
	{"unspecified ecosystem keeps everything", unknown, "Whatever", "V1.0", "Whatever", "V1.0", false},
	{"empty input is total", pypi, "", "", "", "", false},
}

func TestIdentityCases(t *testing.T) {
	for _, test := range identityCases {
		t.Run(test.name, func(t *testing.T) {
			pv := NewPackageVersionFromParts(test.ecosystem, test.rawName, test.rawVersion)

			assert.Equal(t, test.wantName, pv.Name())
			assert.Equal(t, test.wantVersion, pv.Version())
			assert.Equal(t, test.wantParsed, pv.VersionParsed())
			assert.Equal(t, test.rawName, pv.RawName())
			assert.Equal(t, test.rawVersion, pv.RawVersion())
			assert.Equal(t, IdentityRuleVersion(test.ecosystem), pv.RuleVersion())

			again := NewPackageVersionFromParts(test.ecosystem, pv.Name(), pv.Version())
			assert.True(t, pv.Equal(again), "folding a canonical value must be a no-op")
		})
	}
}

// identityDistinctCase is a pair the registry treats as two packages or two
// versions. A fold that merges them is wrong even when it is idempotent.
type identityDistinctCase struct {
	name string
	a    identitySide
	b    identitySide
}

type identitySide struct {
	ecosystem packagev1.Ecosystem
	name      string
	version   string
}

var identityDistinctCases = []identityDistinctCase{
	{
		"pypi separators make another package",
		identitySide{pypi, "datatoolkit", "1"},
		identitySide{pypi, "data-tool-kit", "1"},
	},
	{"pypi pre release is another version", identitySide{pypi, "pkg", "1.0"}, identitySide{pypi, "pkg", "1.0rc1"}},
	{"pypi post release is another version", identitySide{pypi, "pkg", "1.0"}, identitySide{pypi, "pkg", "1.0.post1"}},
	{"pypi dev release is another version", identitySide{pypi, "pkg", "1.0"}, identitySide{pypi, "pkg", "1.0.dev0"}},
	{
		"pypi local segment is another version",
		identitySide{pypi, "pkg", "1.0"},
		identitySide{pypi, "pkg", "1.0+local.1"},
	},
	{"pypi epoch is another version", identitySide{pypi, "pkg", "1!2"}, identitySide{pypi, "pkg", "2!1.0"}},
	{"pypi inner zero is another version", identitySide{pypi, "pkg", "1.0"}, identitySide{pypi, "pkg", "1.0.1"}},
	{"pypi invalid spelling matches only itself", identitySide{pypi, "pkg", "1.0"}, identitySide{pypi, "pkg", "1.0_1"}},
	{
		"npm case makes another package",
		identitySide{npm, "JSONStream", "1.0.3"},
		identitySide{npm, "jsonstream", "1.0.3"},
	},
	{
		"npm scope case makes another package",
		identitySide{npm, "@Vue/Reactivity", "1"},
		identitySide{npm, "@vue/reactivity", "1"},
	},
	{"npm v prefix is another version", identitySide{npm, "express", "1.0.3"}, identitySide{npm, "express", "v1.0.3"}},
	{
		"cargo hyphen and underscore differ",
		identitySide{cargo, "tokio-util", "1"},
		identitySide{cargo, "tokio_util", "1"},
	},
	{
		"rubygems trailing zero differs",
		identitySide{rubygems, "rails", "7.0"},
		identitySide{rubygems, "rails", "7.0.0"},
	},
	{
		"go case differs",
		identitySide{golang, "github.com/safedep/Vet", "v1"},
		identitySide{golang, "github.com/safedep/vet", "v1"},
	},
	{
		"github ref case differs",
		identitySide{github, "actions/checkout", "v4"},
		identitySide{github, "actions/checkout", "V4"},
	},
	{
		"github owner case differs until its rule ships",
		identitySide{github, "Actions/checkout", "v4"},
		identitySide{github, "actions/checkout", "v4"},
	},
	{
		"rubygems case differs until its rule ships",
		identitySide{rubygems, "Rails", "7"},
		identitySide{rubygems, "rails", "7"},
	},
	{
		"packagist case differs until its rule ships",
		identitySide{packagist, "Monolog/Monolog", "3"},
		identitySide{packagist, "monolog/monolog", "3"},
	},
	{
		"gitlab path case differs",
		identitySide{gitlab, "Inkscape/Inkscape", "1.2"},
		identitySide{gitlab, "inkscape/inkscape", "1.2"},
	},
	{
		"packagist vendor differs",
		identitySide{packagist, "vendor-a/library", "1.0.0"},
		identitySide{packagist, "vendor-b/library", "1.0.0"},
	},
	{
		"same spelling in another ecosystem differs",
		identitySide{pypi, "requests", "1"},
		identitySide{npm, "requests", "1"},
	},
}

func TestIdentityDistinctCases(t *testing.T) {
	for _, test := range identityDistinctCases {
		t.Run(test.name, func(t *testing.T) {
			a := NewPackageVersionFromParts(test.a.ecosystem, test.a.name, test.a.version)
			b := NewPackageVersionFromParts(test.b.ecosystem, test.b.name, test.b.version)

			assert.False(t, a.Equal(b), "%v and %v must stay two identities", test.a, test.b)
			assert.NotEqual(t, a.Key(), b.Key())
		})
	}
}
