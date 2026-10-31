package pbidentity

import (
	"fmt"
	"sync"
	"testing"
	"time"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/safedep/dry/api/pb"
	"github.com/safedep/dry/policy/match"
)

var now = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func blockRule(id, purl, vers string) match.Rule {
	return match.Rule{ID: id, Effect: match.EffectBlock, PURL: purl, Vers: vers, Reason: "test"}
}

func trustRule(id, purl, vers string) match.Rule {
	return match.Rule{
		ID: id, Effect: match.EffectTrust, PURL: purl, Vers: vers, Reason: "test",
		Scopes: []match.Scope{match.ScopeMalware}, ExpiresAt: now.Add(time.Hour),
	}
}

func decide(t *testing.T, rule match.Rule, purl string) match.Effect {
	t.Helper()

	ix, dropped := match.NewIndex([]match.Rule{rule}, Identity{})
	require.Empty(t, dropped)

	pkg, err := Identity{}.Package(purl)
	require.NoError(t, err)

	return ix.Decide(match.Query{Package: pkg, Control: match.ScopeMalware, Verdict: match.VerdictMalware, Now: now}).Effect
}

func TestCanonicalMatch(t *testing.T) {
	cases := []struct {
		name string
		rule match.Rule
		purl string
		want match.Effect
	}{
		{name: "npm scope prefix", rule: blockRule("r_1", "pkg:npm/@acme/*", ""), purl: "pkg:npm/@acme/lib@1.0.0", want: match.EffectBlock},
		{name: "npm scope prefix boundary", rule: blockRule("r_1", "pkg:npm/@acme/*", ""), purl: "pkg:npm/@acmex/x@1.0.0", want: match.EffectNone},
		{name: "maven dot prefix", rule: blockRule("r_1", "pkg:maven/org.apache.*", ""), purl: "pkg:maven/org.apache.commons/commons-lang3@3.0", want: match.EffectBlock},
		{name: "maven dot prefix boundary", rule: blockRule("r_1", "pkg:maven/org.apache.*", ""), purl: "pkg:maven/org.apachex/x@1.0", want: match.EffectNone},
		{name: "pypi dot prefix folds", rule: blockRule("r_1", "pkg:pypi/foo.*", ""), purl: "pkg:pypi/Foo_Bar@1.0", want: match.EffectBlock},
		{name: "pypi name folds", rule: blockRule("r_1", "pkg:pypi/Foo_Bar", ""), purl: "pkg:pypi/foo.bar@1.0", want: match.EffectBlock},
		{name: "pypi version folds", rule: blockRule("r_1", "pkg:pypi/foo@1.0", ""), purl: "pkg:pypi/foo@1.0.0", want: match.EffectBlock},
		{name: "pypi range", rule: blockRule("r_1", "pkg:pypi/foo", "vers:pypi/>=1.0|<2.0"), purl: "pkg:pypi/foo@1.5rc1", want: match.EffectBlock},
		{name: "pypi range miss", rule: blockRule("r_1", "pkg:pypi/foo", "vers:pypi/>=1.0|<2.0"), purl: "pkg:pypi/foo@2.0", want: match.EffectNone},
		{name: "npm semver range with prerelease", rule: blockRule("r_1", "pkg:npm/evil", "vers:npm/>=1.2.0|<2.0.0"), purl: "pkg:npm/evil@2.0.0-rc.1", want: match.EffectBlock},
		{name: "github action folds case", rule: blockRule("r_1", "pkg:githubactions/Evil/Action", ""), purl: "pkg:githubactions/evil/action@v1", want: match.EffectBlock},
		{name: "github action ref has no order: block matches", rule: blockRule("r_1", "pkg:githubactions/evil/action", "vers:githubactions/>=v2"), purl: "pkg:githubactions/evil/action@v1", want: match.EffectBlock},
		{name: "trust in range", rule: trustRule("r_1", "pkg:npm/fp", "vers:npm/>=1.0.0|<1.1.0"), purl: "pkg:npm/fp@1.0.9", want: match.EffectTrust},
		{name: "nuget name folds case", rule: blockRule("r_1", "pkg:nuget/Microsoft.Evil", ""), purl: "pkg:nuget/microsoft.evil@1.0.0", want: match.EffectBlock},
		{name: "nuget prefix folds case", rule: blockRule("r_1", "pkg:nuget/Microsoft.*", ""), purl: "pkg:nuget/microsoft.evil@1.0.0", want: match.EffectBlock},
		{name: "nuget prefix boundary", rule: blockRule("r_1", "pkg:nuget/Microsoft.*", ""), purl: "pkg:nuget/microsoftx.evil@1.0.0", want: match.EffectNone},
		{name: "pip purl with a pypi range", rule: blockRule("r_1", "pkg:pip/foo", "vers:pypi/>=1.0|<2.0"), purl: "pkg:pypi/foo@3.0", want: match.EffectNone},
		{name: "pypi purl with a pip range", rule: trustRule("r_1", "pkg:pypi/foo", "vers:pip/>=1.0|<2.0"), purl: "pkg:pypi/foo@1.5", want: match.EffectTrust},
		{name: "golang purl with a go range", rule: blockRule("r_1", "pkg:golang/github.com/acme/lib", "vers:go/>=v1.0.0|<v2.0.0"), purl: "pkg:golang/github.com/acme/lib@v2.1.0", want: match.EffectNone},
		{name: "npm is case-sensitive", rule: blockRule("r_1", "pkg:npm/JSONStream", ""), purl: "pkg:npm/jsonstream@1.0.0", want: match.EffectNone},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, decide(t, tc.rule, tc.purl))
		})
	}
}

func TestVersRangeChecks(t *testing.T) {
	cases := []struct {
		name string
		rule match.Rule
	}{
		{name: "pypi range on an npm purl", rule: trustRule("r_1", "pkg:npm/fp", "vers:pypi/>=1.0")},
		{name: "pypi bounds out of order", rule: trustRule("r_1", "pkg:pypi/fp", "vers:pypi/>=2.0|<1.0")},
		{name: "pypi bounds equal by order", rule: trustRule("r_1", "pkg:pypi/fp", "vers:pypi/>=1.0|<1.0.0")},
		{name: "prefix bounds out of order", rule: blockRule("r_1", "pkg:npm/@acme/*", "vers:npm/>=2.0.0|<1.0.0")},
		{name: "github action range has no order", rule: blockRule("r_1", "pkg:githubactions/evil/action", "vers:githubactions/>=v1|<v2")},
		{name: "github action single bound has no order", rule: blockRule("r_1", "pkg:githubactions/actions/checkout", "vers:githubactions/>=v4")},
		{name: "github action trust with a single bound has no order", rule: trustRule("r_1", "pkg:githubactions/fp/action", "vers:githubactions/>=v2")},
		{name: "github action point has no order", rule: trustRule("r_1", "pkg:githubactions/fp/action", "vers:githubactions/=v4")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, match.Validate(tc.rule, Identity{}), match.ErrInvalidVers)
		})
	}

	assert.NoError(t, match.Validate(blockRule("r_1", "pkg:pip/foo", "vers:pypi/>=1.0|<2.0"), Identity{}))
	assert.NoError(t, match.Validate(blockRule("r_1", "pkg:maven/org.apache.*", "vers:maven/>=1.0|<2.0"), Identity{}))
}

func TestVersionCacheIsBounded(t *testing.T) {
	cache := &versionCache{versions: map[packagev1.Ecosystem]map[string]parsedVersion{}}
	npm := packagev1.Ecosystem_ECOSYSTEM_NPM

	for i := range maxCachedVersions + 10 {
		v, err := cache.parse(npm, fmt.Sprintf("1.0.%d", i))
		require.NoError(t, err)

		want, err := pb.ParseVersion(npm, fmt.Sprintf("1.0.%d", i))
		require.NoError(t, err)

		cmp, err := v.Compare(want)
		require.NoError(t, err)
		require.Zero(t, cmp)
	}

	assert.Equal(t, maxCachedVersions, cache.size)
	assert.Len(t, cache.versions[npm], maxCachedVersions)

	_, err := cache.parse(npm, "")
	assert.Error(t, err, "an error is not cached past the bound")
}

func TestVersionCacheIsSafeForConcurrentUse(t *testing.T) {
	cache := &versionCache{versions: map[packagev1.Ecosystem]map[string]parsedVersion{}}
	ecosystems := []packagev1.Ecosystem{packagev1.Ecosystem_ECOSYSTEM_NPM, packagev1.Ecosystem_ECOSYSTEM_PYPI}

	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				eco := ecosystems[(g+i)%len(ecosystems)]
				v, err := cache.parse(eco, fmt.Sprintf("1.%d.0", i%50))
				if assert.NoError(t, err) {
					want, err := pb.ParseVersion(eco, fmt.Sprintf("1.%d.0", i%50))
					require.NoError(t, err)
					cmp, err := v.Compare(want)
					assert.NoError(t, err)
					assert.Zero(t, cmp)
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 100, cache.size)
}

func TestPackageIsSafeForConcurrentUse(t *testing.T) {
	pkg, err := Identity{}.Package("pkg:pypi/foo@1.5")
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmp, err := pkg.CompareVersion("2.0")
			assert.NoError(t, err)
			assert.Equal(t, -1, cmp)
		}()
	}
	wg.Wait()
}

func TestDroppedSelectors(t *testing.T) {
	cases := []struct {
		name string
		rule match.Rule
	}{
		{name: "unknown ecosystem", rule: blockRule("r_1", "pkg:unknownthing/x", "")},
		{name: "prefix with a version", rule: blockRule("r_1", "pkg:npm/x@1.*", "")},
		{name: "prefix with no boundary", rule: blockRule("r_1", "pkg:npm/ev*", "")},
		{name: "namespace in pypi", rule: blockRule("r_1", "pkg:pypi/acme/x", "")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, dropped := match.NewIndex([]match.Rule{tc.rule}, Identity{})
			require.Len(t, dropped, 1)
			assert.ErrorIs(t, dropped[0].Err, match.ErrInvalidSelector)
		})
	}
}

func benchIndex(b testing.TB) *match.Index {
	rules := make([]match.Rule, 0, 10_000)
	for i := range 10_000 {
		id := fmt.Sprintf("r_%07d", i)
		switch i % 10 {
		case 0:
			rules = append(rules, blockRule(id, fmt.Sprintf("pkg:npm/@scope%d/*", i), ""))
		case 1, 2, 3:
			rules = append(rules, trustRule(id, fmt.Sprintf("pkg:npm/pkg%d", i), "vers:npm/>=1.0.0|<2.0.0"))
		case 4, 5:
			rules = append(rules, blockRule(id, fmt.Sprintf("pkg:pypi/pkg%d", i), "vers:pypi/>=3.0|<4.0|>=5.0"))
		default:
			rules = append(rules, blockRule(id, fmt.Sprintf("pkg:npm/pkg%d", i), ""))
		}
	}

	ix, dropped := match.NewIndex(rules, Identity{})
	require.Empty(b, dropped)

	return ix
}

func benchQueries(b testing.TB) map[string]match.Query {
	q := func(purl string) match.Query {
		pkg, err := Identity{}.Package(purl)
		require.NoError(b, err)
		return match.Query{Package: pkg, Control: match.ScopeMalware, Verdict: match.VerdictMalware, Now: now}
	}

	return map[string]match.Query{
		"trust hit":   q("pkg:npm/pkg5001@1.5.0"),
		"range block": q("pkg:pypi/pkg5004@3.5"),
		"prefix hit":  q("pkg:npm/@scope5000/lib@1.5.0"),
		"miss":        q("pkg:npm/clean@1.5.0"),
	}
}

// BenchmarkDecide measures one check at 10,000 rules with the identity of
// dry/api/pb. The target is under 10 µs for each check.
func BenchmarkDecide(b *testing.B) {
	ix := benchIndex(b)
	for name, q := range benchQueries(b) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = ix.Decide(q)
			}
		})
	}
}

// BenchmarkParseAndDecide measures a request of a tool: it parses the PURL of
// the package and checks it once. So the cost holds the first parse of the
// package version.
func BenchmarkParseAndDecide(b *testing.B) {
	ix := benchIndex(b)
	for name, purl := range map[string]string{
		"trust hit":   "pkg:npm/pkg5001@1.5.0",
		"range block": "pkg:pypi/pkg5004@3.5",
		"miss":        "pkg:npm/clean@1.5.0",
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				pkg, err := Identity{}.Package(purl)
				if err != nil {
					b.Fatal(err)
				}
				_ = ix.Decide(match.Query{Package: pkg, Control: match.ScopeMalware, Verdict: match.VerdictMalware, Now: now})
			}
		})
	}
}

// TestDecideIsFast is the gate on the benchmark: one check at 10,000 rules
// takes less than 10 µs. The measured cost is far below the gate, so the
// test stays stable on a slow machine.
func TestDecideIsFast(t *testing.T) {
	if testing.Short() {
		t.Skip("benchmark gate")
	}

	ix := benchIndex(t)
	for name, q := range benchQueries(t) {
		res := testing.Benchmark(func(b *testing.B) {
			for b.Loop() {
				_ = ix.Decide(q)
			}
		})

		perCheck := time.Duration(res.NsPerOp())
		assert.Less(t, perCheck, 10*time.Microsecond, "%s takes %s for each check", name, perCheck)
	}
}
