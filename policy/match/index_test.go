package match

import (
	"go/build"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIndexDrops(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		wantErr error
	}{
		{name: "no effect", rule: Rule{ID: "r_1", PURL: "pkg:npm/x"}, wantErr: ErrUnknownEffect},
		{name: "later effect", rule: Rule{ID: "r_1", Effect: Effect(7), PURL: "pkg:npm/x"}, wantErr: ErrUnknownEffect},
		{name: "trust with no id", rule: trust("", "pkg:npm/x", ScopeMalware), wantErr: ErrNoRuleID},
		{name: "trust with no expiry", rule: withExpiry(trust("r_1", "pkg:npm/x", ScopeMalware), time.Time{}), wantErr: ErrNoExpiry},
		{name: "trust with only not equal", rule: withVers(trust("r_1", "pkg:npm/x", ScopeMalware), "vers:npm/!=1.0.0"), wantErr: ErrInvalidVers},
		{name: "trust with bounds out of order", rule: withVers(trust("r_1", "pkg:npm/x", ScopeMalware), "vers:npm/>=2.0.0|<1.0.0"), wantErr: ErrInvalidVers},
		{name: "trust with bounds that do not compare", rule: withVers(trust("r_1", "pkg:npm/x", ScopeMalware), "vers:npm/>=beta|<1.0.0"), wantErr: ErrInvalidVers},
		{name: "trust with no scope", rule: trust("r_1", "pkg:npm/x"), wantErr: ErrNoKnownScope},
		{name: "trust with only an unknown scope", rule: trust("r_1", "pkg:npm/x", Scope(99)), wantErr: ErrNoKnownScope},
		{name: "trust with a prefix", rule: trust("r_1", "pkg:npm/@acme/*", ScopeMalware), wantErr: ErrTrustPrefix},
		{name: "trust with an encoded star", rule: trust("r_1", "pkg:npm/@acme/%2A", ScopeMalware), wantErr: ErrTrustPrefix},
		{name: "trust with a bad vers", rule: withVers(trust("r_1", "pkg:npm/x", ScopeMalware), "vers:npm/>=1|>=2"), wantErr: ErrInvalidVers},
		{name: "trust with a vers of another type", rule: withVers(trust("r_1", "pkg:npm/x", ScopeMalware), "vers:pypi/>=1"), wantErr: ErrInvalidVers},
		{name: "trust with a version and a vers", rule: withVers(trust("r_1", "pkg:npm/x@1.0.0", ScopeMalware), "vers:npm/>=1"), wantErr: ErrInvalidVers},
		{name: "block the identity cannot parse", rule: block("r_1", "pkg:unknown/x"), wantErr: ErrInvalidSelector},
		{name: "block prefix the identity cannot parse", rule: block("r_1", "pkg:npm/x*"), wantErr: ErrInvalidSelector},
		{name: "trust the identity cannot parse", rule: trust("r_1", "npm/x", ScopeMalware), wantErr: ErrInvalidSelector},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ix, dropped := NewIndex([]Rule{tc.rule}, testIdentity{})
			require.Len(t, dropped, 1)
			assert.ErrorIs(t, dropped[0].Err, tc.wantErr)
			assert.Equal(t, tc.rule.ID, dropped[0].Rule.ID)
			assert.Empty(t, ix.Rules())
		})
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		rule    Rule
		wantErr error
	}{
		{name: "block", rule: block("r_1", "pkg:npm/evil")},
		{name: "block with a range", rule: withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1.0.0|<2.0.0")},
		{name: "prefix block with a range", rule: withVers(block("r_1", "pkg:npm/@acme/*"), "vers:npm/>=1.0.0|<2.0.0")},
		{name: "trust", rule: trust("r_1", "pkg:npm/fp", ScopeMalware)},
		{name: "block with a bad vers", rule: withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=1|>=2"), wantErr: ErrInvalidVers},
		{name: "block with only not equal", rule: withVers(block("r_1", "pkg:npm/evil"), "vers:npm/!=1.0.0"), wantErr: ErrInvalidVers},
		{name: "block with bounds out of order", rule: withVers(block("r_1", "pkg:npm/evil"), "vers:npm/>=2.0.0|<1.0.0"), wantErr: ErrInvalidVers},
		{name: "prefix block with bounds out of order", rule: withVers(block("r_1", "pkg:npm/@acme/*"), "vers:npm/>=2.0.0|<1.0.0"), wantErr: ErrInvalidVers},
		{name: "block with a vers of another type", rule: withVers(block("r_1", "pkg:npm/evil"), "vers:pypi/>=1.0.0"), wantErr: ErrInvalidVers},
		{name: "block with a version and a vers", rule: withVers(block("r_1", "pkg:npm/evil@1.0.0"), "vers:npm/>=1.0.0"), wantErr: ErrInvalidVers},
		{name: "block the identity cannot parse", rule: block("r_1", "pkg:unknown/x"), wantErr: ErrInvalidSelector},
		{name: "trust with no expiry", rule: withExpiry(trust("r_1", "pkg:npm/fp", ScopeMalware), time.Time{}), wantErr: ErrNoExpiry},
		{name: "unknown effect", rule: Rule{ID: "r_1", PURL: "pkg:npm/x"}, wantErr: ErrUnknownEffect},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.rule, testIdentity{})
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestCompareRulesIsTotal(t *testing.T) {
	base := trust("r_1", "pkg:npm/x", ScopeMalware)
	variants := []func(*Rule){
		func(r *Rule) { r.Effect = EffectBlock },
		func(r *Rule) { r.PURL = "pkg:npm/y" },
		func(r *Rule) { r.Vers = "vers:npm/>=1.0.0" },
		func(r *Rule) { r.Scopes = []Scope{ScopeMalware, ScopeCooldown} },
		func(r *Rule) { r.ExpiresAt = r.ExpiresAt.Add(time.Hour) },
		func(r *Rule) { r.Reason = "other" },
		func(r *Rule) { r.Origin.Kind = OriginLocal },
		func(r *Rule) { r.Origin.Path = "/etc/x.yml" },
	}

	for i, change := range variants {
		other := base
		other.Scopes = slices.Clone(base.Scopes)
		change(&other)

		assert.NotZero(t, compareRules(base, other), "variant %d", i)
		assert.Equal(t, -compareRules(base, other), compareRules(other, base), "variant %d", i)
	}
}

func TestNewIndexWithNoIdentity(t *testing.T) {
	ix, dropped := NewIndex([]Rule{block("r_1", "pkg:npm/x")}, nil)
	require.Len(t, dropped, 1)
	assert.ErrorIs(t, dropped[0].Err, ErrInvalidSelector)
	assert.Empty(t, ix.Rules())
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		name string
		rule Rule
		want Rule
	}{
		{
			name: "block with no id gets the id -",
			rule: block("", "pkg:npm/x"),
			want: block(BlockRuleIDNone, "pkg:npm/x"),
		},
		{
			name: "block loses its scopes",
			rule: Rule{ID: "r_1", Effect: EffectBlock, PURL: "pkg:npm/x", Scopes: []Scope{ScopeMalware}},
			want: Rule{ID: "r_1", Effect: EffectBlock, PURL: "pkg:npm/x"},
		},
		{
			name: "block keeps a bad vers",
			rule: withVers(block("r_1", "pkg:npm/x"), "bad"),
			want: withVers(block("r_1", "pkg:npm/x"), "bad"),
		},
		{
			name: "block keeps a prefix",
			rule: block("r_1", "pkg:npm/*"),
			want: block("r_1", "pkg:npm/*"),
		},
		{
			name: "trust keeps only known scopes, in order, once each",
			rule: trust("r_1", "pkg:npm/x", Scope(99), ScopeCooldown, ScopeMalware, ScopeCooldown),
			want: trust("r_1", "pkg:npm/x", ScopeMalware, ScopeCooldown),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.rule)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIndexRules(t *testing.T) {
	ix, dropped := NewIndex([]Rule{
		block("r_c", "pkg:npm/c"),
		trust("r_a", "pkg:npm/a", ScopeMalware),
		block("", "pkg:npm/b"),
		trust("r_x", "pkg:npm/@x/*", ScopeMalware),
	}, testIdentity{})
	require.Len(t, dropped, 1)

	var ids []string
	for _, r := range ix.Rules() {
		ids = append(ids, r.ID)
	}
	assert.Equal(t, []string{BlockRuleIDNone, "r_a", "r_c"}, ids)
}

func TestIndexIsSafeForConcurrentUse(t *testing.T) {
	ix, dropped := NewIndex([]Rule{block("r_1", "pkg:npm/@acme/*"), trust("r_2", "pkg:npm/fp", ScopeMalware)}, testIdentity{})
	require.Empty(t, dropped)

	done := make(chan Effect)
	for range 8 {
		go func() {
			done <- ix.Decide(Query{Package: mustPackage("pkg:npm/@acme/x@1.0.0"), Control: ScopeMalware, Now: time.Now()}).Effect
		}()
	}

	for range 8 {
		assert.Equal(t, EffectBlock, <-done)
	}
}

// TestNoContractImport keeps the rule that match has no import of the
// SafeDep API contract. A tool whose rules forbid that import can use match.
func TestNoContractImport(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	require.NoError(t, err)

	for _, imp := range pkg.Imports {
		assert.False(t, strings.HasPrefix(imp, "buf.build/"), "match imports %s", imp)
		assert.False(t, strings.HasPrefix(imp, "github.com/safedep/"), "match imports %s", imp)
		assert.False(t, strings.HasPrefix(imp, "google.golang.org/protobuf"), "match imports %s", imp)
	}
}
