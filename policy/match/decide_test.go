package match

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

func block(id, purl string) Rule {
	return Rule{ID: id, Effect: EffectBlock, PURL: purl, Reason: "test", Origin: Origin{Kind: OriginCloud}}
}

func trust(id, purl string, scopes ...Scope) Rule {
	return Rule{
		ID: id, Effect: EffectTrust, PURL: purl, Scopes: scopes, Reason: "test",
		ExpiresAt: testNow.Add(24 * time.Hour), Origin: Origin{Kind: OriginCloud},
	}
}

func withVers(r Rule, vers string) Rule {
	r.Vers = vers
	return r
}

func withExpiry(r Rule, at time.Time) Rule {
	r.ExpiresAt = at
	return r
}

func TestDecide(t *testing.T) {
	cases := []struct {
		name       string
		rules      []Rule
		purl       string
		control    Scope
		verdict    Verdict
		now        time.Time
		wantEffect Effect
		wantRule   string
	}{
		{
			name:       "no rule",
			rules:      []Rule{block("r_1", "pkg:npm/evil")},
			purl:       "pkg:npm/good@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "block entry blocks every version",
			rules:      []Rule{block("r_1", "pkg:npm/evil")},
			purl:       "pkg:npm/evil@9.9.9",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "block entry blocks with no verdict",
			rules:      []Rule{block("r_1", "pkg:npm/evil")},
			purl:       "pkg:npm/evil@1.0.0",
			control:    ScopeCooldown,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "block beats trust",
			rules:      []Rule{trust("r_0", "pkg:npm/evil", ScopeMalware), block("r_9", "pkg:npm/evil")},
			purl:       "pkg:npm/evil@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_9",
		},
		{
			name:       "prefix block beats exact trust",
			rules:      []Rule{trust("r_0", "pkg:npm/@acme/lib", ScopeMalware), block("r_9", "pkg:npm/@acme/*")},
			purl:       "pkg:npm/@acme/lib@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_9",
		},
		{
			name:       "malware trust waives a malware verdict",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeMalware)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "cooldown trust waives the cooldown",
			rules:      []Rule{trust("r_1", "pkg:npm/young", ScopeCooldown)},
			purl:       "pkg:npm/young@1.0.0",
			control:    ScopeCooldown,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "cooldown trust does not waive malware",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeCooldown)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "malware trust does not waive the cooldown",
			rules:      []Rule{trust("r_1", "pkg:npm/young", ScopeMalware)},
			purl:       "pkg:npm/young@1.0.0",
			control:    ScopeCooldown,
			wantEffect: EffectNone,
		},
		{
			name:       "vulnerability trust changes no malware decision",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeVulnerability)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "vulnerability trust changes no cooldown decision",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeVulnerability)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeCooldown,
			wantEffect: EffectNone,
		},
		{
			name:       "vulnerability trust waives a vulnerability finding",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeVulnerability)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeVulnerability,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "trust applies before its expiry",
			rules:      []Rule{withExpiry(trust("r_1", "pkg:npm/fp", ScopeMalware), testNow.Add(time.Nanosecond))},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "trust stops at its expiry",
			rules:      []Rule{withExpiry(trust("r_1", "pkg:npm/fp", ScopeMalware), testNow)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "trust after its expiry",
			rules:      []Rule{withExpiry(trust("r_1", "pkg:npm/fp", ScopeMalware), testNow.Add(-time.Hour))},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "block stops at its expiry",
			rules:      []Rule{withExpiry(block("r_1", "pkg:npm/evil"), testNow)},
			purl:       "pkg:npm/evil@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "expired block does not hide a trust",
			rules:      []Rule{withExpiry(block("r_0", "pkg:npm/fp"), testNow), trust("r_1", "pkg:npm/fp", ScopeMalware)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "the lower block id decides",
			rules:      []Rule{block("r_b", "pkg:npm/evil"), block("r_a", "pkg:npm/*")},
			purl:       "pkg:npm/evil@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_a",
		},
		{
			name:       "the lower trust id decides",
			rules:      []Rule{trust("r_b", "pkg:npm/fp", ScopeMalware), trust("r_a", "pkg:npm/fp@1.0.0", ScopeMalware)},
			purl:       "pkg:npm/fp@1.0.0",
			control:    ScopeMalware,
			verdict:    VerdictMalware,
			wantEffect: EffectTrust,
			wantRule:   "r_a",
		},
		{
			name:       "version selector matches its version",
			rules:      []Rule{block("r_1", "pkg:npm/evil@1.2.4")},
			purl:       "pkg:npm/evil@1.2.4",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "version selector does not match another version",
			rules:      []Rule{block("r_1", "pkg:npm/evil@1.2.4")},
			purl:       "pkg:npm/evil@1.2.5",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "version block matches a package with no version",
			rules:      []Rule{block("r_1", "pkg:npm/evil@1.2.4")},
			purl:       "pkg:npm/evil",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "version trust does not match a package with no version",
			rules:      []Rule{trust("r_1", "pkg:npm/fp@1.0.0", ScopeCooldown)},
			purl:       "pkg:npm/fp",
			control:    ScopeCooldown,
			wantEffect: EffectNone,
		},
		{
			name:       "vers block in range",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1.2.0|<2.0.0")},
			purl:       "pkg:npm/evil@1.5.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block out of range",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1.2.0|<2.0.0")},
			purl:       "pkg:npm/evil@2.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "vers trust in range",
			rules:      []Rule{withVers(trust("r_1", "pkg:npm/young", ScopeCooldown), "vers:npm/>=1.0.0|<1.1.0")},
			purl:       "pkg:npm/young@1.0.5",
			control:    ScopeCooldown,
			wantEffect: EffectTrust,
			wantRule:   "r_1",
		},
		{
			name:       "vers trust out of range",
			rules:      []Rule{withVers(trust("r_1", "pkg:npm/young", ScopeCooldown), "vers:npm/>=1.0.0|<1.1.0")},
			purl:       "pkg:npm/young@1.1.0",
			control:    ScopeCooldown,
			wantEffect: EffectNone,
		},
		{
			name:       "vers block matches a version that does not compare",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1.2.0|<2.0.0")},
			purl:       "pkg:npm/evil@beta",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers trust does not match a version that does not compare",
			rules:      []Rule{withVers(trust("r_1", "pkg:npm/young", ScopeCooldown), "vers:npm/>=1.0.0")},
			purl:       "pkg:npm/young@beta",
			control:    ScopeCooldown,
			wantEffect: EffectNone,
		},
		{
			name:       "vers block with bounds out of order matches",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/<2.0.0|>=1.0.0")},
			purl:       "pkg:npm/evil@1.5.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block with bounds out of order matches outside both bounds",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=2.0.0|<1.0.0")},
			purl:       "pkg:npm/evil@3.0.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block with only not equal matches every version",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/!=1.0.0")},
			purl:       "pkg:npm/evil@2.0.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "no control gives no trust",
			rules:      []Rule{trust("r_1", "pkg:npm/fp", ScopeMalware, ScopeCooldown, ScopeVulnerability)},
			purl:       "pkg:npm/fp@1.0.0",
			verdict:    VerdictMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "no control still blocks",
			rules:      []Rule{block("r_1", "pkg:npm/evil")},
			purl:       "pkg:npm/evil@1.0.0",
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block that does not parse matches every version",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1.0.0|>=2.0.0")},
			purl:       "pkg:npm/evil@0.1.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block with another type matches every version",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil"), "vers:pypi/>=1.0.0")},
			purl:       "pkg:npm/evil@0.1.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "vers block with a version in the purl matches every version",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/evil@1.0.0"), "vers:npm/>=2.0.0")},
			purl:       "pkg:npm/evil@0.1.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "prefix block matches under its boundary",
			rules:      []Rule{block("r_1", "pkg:npm/@acme/*")},
			purl:       "pkg:npm/@acme/lib@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_1",
		},
		{
			name:       "prefix block with a slash boundary",
			rules:      []Rule{block("r_1", "pkg:npm/@acme/*")},
			purl:       "pkg:npm/@acmex/x@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "prefix block with a dot boundary",
			rules:      []Rule{block("r_1", "pkg:maven/org.apache.*")},
			purl:       "pkg:maven/org.apachex/x@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "prefix block in another ecosystem",
			rules:      []Rule{block("r_1", "pkg:npm/*")},
			purl:       "pkg:pypi/x@1.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "prefix block with a range",
			rules:      []Rule{withVers(block("r_1", "pkg:npm/@acme/*"), "vers:npm/<2.0.0")},
			purl:       "pkg:npm/@acme/lib@2.0.0",
			control:    ScopeMalware,
			wantEffect: EffectNone,
		},
		{
			name:       "two prefix lengths",
			rules:      []Rule{block("r_2", "pkg:npm/@acme/*"), withVers(block("r_1", "pkg:npm/*"), "vers:npm/<1.0.0")},
			purl:       "pkg:npm/@acme/lib@2.0.0",
			control:    ScopeMalware,
			wantEffect: EffectBlock,
			wantRule:   "r_2",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix, dropped := NewIndex(tc.rules, testIdentity{})
			require.Empty(t, dropped)

			now := tc.now
			if now.IsZero() {
				now = testNow
			}

			got := ix.Decide(Query{Package: mustPackage(tc.purl), Control: tc.control, Verdict: tc.verdict, Now: now})
			assert.Equal(t, tc.wantEffect, got.Effect)
			assert.Equal(t, tc.wantRule, got.Rule.ID)
		})
	}
}

// TestVerifiedMalwareGuard checks that no trust entry waives verified
// malware, for each scope, each set of scopes and each control.
func TestVerifiedMalwareGuard(t *testing.T) {
	scopeSets := [][]Scope{
		{ScopeMalware},
		{ScopeCooldown},
		{ScopeVulnerability},
		{ScopeMalware, ScopeCooldown},
		{ScopeMalware, ScopeCooldown, ScopeVulnerability},
	}
	controls := []Scope{ScopeMalware, ScopeCooldown, ScopeVulnerability}

	for _, scopes := range scopeSets {
		for _, control := range controls {
			t.Run(fmt.Sprintf("scopes %v control %v", scopes, control), func(t *testing.T) {
				ix, dropped := NewIndex([]Rule{
					trust("r_1", "pkg:npm/evil", scopes...),
					withVers(trust("r_2", "pkg:npm/evil", scopes...), "vers:npm/>=0.0.1"),
					trust("r_3", "pkg:npm/evil@1.0.0", scopes...),
				}, testIdentity{})
				require.Empty(t, dropped)

				got := ix.Decide(Query{
					Package: mustPackage("pkg:npm/evil@1.0.0"),
					Control: control,
					Verdict: VerdictVerifiedMalware,
					Now:     testNow,
				})
				assert.Equal(t, EffectNone, got.Effect)
				assert.Empty(t, got.Rule.ID)
			})
		}
	}
}

func TestVerifiedMalwareStillBlocks(t *testing.T) {
	ix, dropped := NewIndex([]Rule{trust("r_0", "pkg:npm/evil", ScopeMalware), block("r_1", "pkg:npm/evil")}, testIdentity{})
	require.Empty(t, dropped)

	got := ix.Decide(Query{Package: mustPackage("pkg:npm/evil@1.0.0"), Control: ScopeMalware, Verdict: VerdictVerifiedMalware, Now: testNow})
	assert.Equal(t, EffectBlock, got.Effect)
	assert.Equal(t, "r_1", got.Rule.ID)
}

func TestDecideZeroNowUsesTheClock(t *testing.T) {
	past := withExpiry(trust("r_1", "pkg:npm/fp", ScopeMalware), time.Now().Add(-time.Minute))
	ix, dropped := NewIndex([]Rule{past}, testIdentity{})
	require.Empty(t, dropped)

	got := ix.Decide(Query{Package: mustPackage("pkg:npm/fp@1.0.0"), Control: ScopeMalware, Verdict: VerdictMalware})
	assert.Equal(t, EffectNone, got.Effect)
}

func TestDecideWithNoPackage(t *testing.T) {
	ix, _ := NewIndex([]Rule{block("r_1", "pkg:npm/*")}, testIdentity{})
	assert.Equal(t, Decision{}, ix.Decide(Query{Control: ScopeMalware}))

	var nilIndex *Index
	assert.Equal(t, Decision{}, nilIndex.Decide(Query{Package: mustPackage("pkg:npm/x@1.0.0")}))
}

func TestDecideReturnsTheRule(t *testing.T) {
	r := block("r_1", "pkg:npm/evil")
	r.Reason = "IR-2291 credential stealer"
	r.Origin = Origin{Kind: OriginLocal, Path: "/etc/safedep/pmg/config.yml"}

	ix, dropped := NewIndex([]Rule{r}, testIdentity{})
	require.Empty(t, dropped)

	got := ix.Decide(Query{Package: mustPackage("pkg:npm/evil@1.0.0"), Control: ScopeMalware, Now: testNow})
	assert.Equal(t, r, got.Rule)
}

func TestDecideAndRulesCopyTheScopes(t *testing.T) {
	ix, dropped := NewIndex([]Rule{trust("r_1", "pkg:npm/fp", ScopeMalware)}, testIdentity{})
	require.Empty(t, dropped)

	q := Query{Package: mustPackage("pkg:npm/fp@1.0.0"), Control: ScopeMalware, Verdict: VerdictMalware, Now: testNow}
	ix.Decide(q).Rule.Scopes[0] = ScopeCooldown
	ix.Rules()[0].Scopes[0] = ScopeCooldown

	assert.Equal(t, EffectTrust, ix.Decide(q).Effect)
	assert.Equal(t, []Scope{ScopeMalware}, ix.Rules()[0].Scopes)
}
