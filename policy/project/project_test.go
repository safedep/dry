package project

import (
	"testing"
	"time"

	corev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/core/v1"
	optionsv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/options/v1"
	packageadmissionv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/packageadmission/v1"
	recordv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/record/v1"
	scangatev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/scangate/v1"
	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/safedep/dry/policy/match"
)

const (
	testTenant    = "platform-eng.acme.safedep.io"
	testMachineID = "m_5d1c"
)

var (
	issuedAt = time.Date(2026, 10, 30, 10, 2, 11, 0, time.UTC)
	expires  = time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)

	endpointExpect = Expect{
		Tenant:      testTenant,
		SubjectKind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
		MachineID:   testMachineID,
	}
)

// newPolicy returns a resolved policy of an endpoint with a cooldown of
// three days and no package rule.
func newPolicy() *specv1.ResolvedPolicy {
	return specv1.ResolvedPolicy_builder{
		Header: specv1.ResolvedPolicyHeader_builder{
			Tenant: testTenant,
			Subject: specv1.Subject_builder{
				Kind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
				Id:   "ep_9c1x4kq7dm",
			}.Build(),
			Revision:              48,
			DefaultPolicyRevision: 12,
			IssuedAt:              timestamppb.New(issuedAt),
			MatchedOverrides: []*specv1.MatchedOverride{
				specv1.MatchedOverride_builder{OverrideId: "ov_1", Name: "payments", Priority: 10}.Build(),
			},
			MachineId: testMachineID,
		}.Build(),
		Spec: specv1.PolicySpec_builder{
			PackageAdmission: packageadmissionv1.PackageAdmission_builder{
				Cooldown: packageadmissionv1.Cooldown_builder{MinAge: durationpb.New(72 * time.Hour)}.Build(),
			}.Build(),
		}.Build(),
		FieldSources: []*specv1.FieldSource{
			specv1.FieldSource_builder{
				Path:       "package_admission.cooldown.min_age",
				Layer:      recordv1.Layer_LAYER_OVERRIDE,
				OverrideId: "ov_1",
			}.Build(),
		},
	}.Build()
}

func encode(t *testing.T, policy *specv1.ResolvedPolicy) []byte {
	t.Helper()

	raw, err := proto.Marshal(policy)
	require.NoError(t, err)
	return raw
}

// addUnknown adds a varint field with a number that the contract does not
// know.
func addUnknown(m proto.Message, num protowire.Number) {
	b := protowire.AppendTag(nil, num, protowire.VarintType)
	b = protowire.AppendVarint(b, 1)

	r := m.ProtoReflect()
	r.SetUnknown(append(r.GetUnknown(), b...))
}

func decode(t *testing.T, policy *specv1.ResolvedPolicy) *Projection {
	t.Helper()

	p, err := Decode(encode(t, policy), endpointExpect)
	require.NoError(t, err)
	return p
}

func TestDecode(t *testing.T) {
	p := decode(t, newPolicy())

	assert.Equal(t, Header{
		Tenant:                testTenant,
		SubjectKind:           specv1.SubjectKind_SUBJECT_KIND_ENDPOINT,
		SubjectID:             "ep_9c1x4kq7dm",
		MachineID:             testMachineID,
		Revision:              48,
		DefaultPolicyRevision: 12,
		IssuedAt:              issuedAt,
		MatchedOverrides:      []MatchedOverride{{ID: "ov_1", Name: "payments", Priority: 10}},
	}, p.Header)

	assert.Equal(t, map[string]any{"package_admission.cooldown.min_age": 72 * time.Hour}, p.Values)
	assert.Equal(t, map[SourceKey]Source{
		{Path: "package_admission.cooldown.min_age"}: {Layer: recordv1.Layer_LAYER_OVERRIDE, OverrideID: "ov_1"},
	}, p.Sources)
	assert.Empty(t, p.Rules)
	assert.Empty(t, p.Exceptions)
	assert.Empty(t, p.FailedCapabilities)
}

func TestDecodeZeroCooldown(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().GetPackageAdmission().GetCooldown().SetMinAge(durationpb.New(0))

	p := decode(t, policy)
	assert.Equal(t, time.Duration(0), p.Values["package_admission.cooldown.min_age"])
}

func TestDecodeUnsetField(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().ClearPackageAdmission()

	p := decode(t, policy)
	assert.Empty(t, p.Values)
}

func TestDecodeWithNoSpec(t *testing.T) {
	policy := newPolicy()
	policy.ClearSpec()

	_, err := Decode(encode(t, policy), endpointExpect)
	assert.ErrorIs(t, err, ErrDecode)
}

func TestDecodeNotAPolicy(t *testing.T) {
	_, err := Decode([]byte{0xff, 0xff, 0xff}, endpointExpect)
	assert.ErrorIs(t, err, ErrDecode)
}

func TestHeaderCheck(t *testing.T) {
	projectExpect := Expect{Tenant: testTenant, SubjectKind: specv1.SubjectKind_SUBJECT_KIND_PROJECT, ProjectID: "prj_1"}

	cases := []struct {
		name    string
		change  func(*specv1.ResolvedPolicy)
		expect  Expect
		wantErr bool
	}{
		{name: "server endpoint id", expect: endpointExpect},
		{
			name:   "machine endpoint id",
			change: func(p *specv1.ResolvedPolicy) { p.GetHeader().GetSubject().SetId("machine:" + testMachineID) },
			expect: endpointExpect,
		},
		{
			name:   "no login skips the tenant",
			change: func(p *specv1.ResolvedPolicy) { p.GetHeader().SetTenant("other.acme.safedep.io") },
			expect: Expect{SubjectKind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT, MachineID: testMachineID},
		},
		{
			name:    "other tenant",
			change:  func(p *specv1.ResolvedPolicy) { p.GetHeader().SetTenant("other.acme.safedep.io") },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "other machine",
			change:  func(p *specv1.ResolvedPolicy) { p.GetHeader().SetMachineId("m_other") },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "no machine id in the header",
			change:  func(p *specv1.ResolvedPolicy) { p.GetHeader().SetMachineId("") },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "no machine id to expect",
			expect:  Expect{Tenant: testTenant, SubjectKind: specv1.SubjectKind_SUBJECT_KIND_ENDPOINT},
			wantErr: true,
		},
		{
			name: "project policy on an endpoint",
			change: func(p *specv1.ResolvedPolicy) {
				p.GetHeader().GetSubject().SetKind(specv1.SubjectKind_SUBJECT_KIND_PROJECT)
			},
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "no subject",
			change:  func(p *specv1.ResolvedPolicy) { p.GetHeader().ClearSubject() },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "no issued_at",
			change:  func(p *specv1.ResolvedPolicy) { p.GetHeader().ClearIssuedAt() },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name:    "no header",
			change:  func(p *specv1.ResolvedPolicy) { p.ClearHeader() },
			expect:  endpointExpect,
			wantErr: true,
		},
		{
			name: "project",
			change: func(p *specv1.ResolvedPolicy) {
				p.GetHeader().SetSubject(specv1.Subject_builder{Kind: specv1.SubjectKind_SUBJECT_KIND_PROJECT, Id: "prj_1"}.Build())
			},
			expect: projectExpect,
		},
		{
			name: "other project",
			change: func(p *specv1.ResolvedPolicy) {
				p.GetHeader().SetSubject(specv1.Subject_builder{Kind: specv1.SubjectKind_SUBJECT_KIND_PROJECT, Id: "prj_2"}.Build())
			},
			expect:  projectExpect,
			wantErr: true,
		},
		{
			name: "unknown subject kind",
			change: func(p *specv1.ResolvedPolicy) {
				p.GetHeader().GetSubject().SetKind(specv1.SubjectKind(9))
			},
			expect:  Expect{Tenant: testTenant, SubjectKind: specv1.SubjectKind(9), MachineID: testMachineID},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := newPolicy()
			if tc.change != nil {
				tc.change(policy)
			}

			p, err := Decode(encode(t, policy), tc.expect)
			if tc.wantErr {
				assert.ErrorIs(t, err, ErrHeader)
				assert.Nil(t, p)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, uint64(48), p.Header.Revision)
		})
	}
}

func TestUnknownFields(t *testing.T) {
	policy := newPolicy()
	addUnknown(policy, 40)
	addUnknown(policy.GetHeader(), 41)
	addUnknown(policy.GetSpec(), 9)
	addUnknown(policy.GetSpec().GetPackageAdmission(), 7)
	addUnknown(policy.GetSpec().GetPackageAdmission(), 7)
	addUnknown(policy.GetSpec().GetPackageAdmission().GetCooldown(), 5)
	addUnknown(policy.GetFieldSources()[0], 6)

	p := decode(t, policy)

	var paths []string
	for _, e := range p.Exceptions {
		assert.Equal(t, ExceptionUnknownField, e.Kind)
		paths = append(paths, e.Path)
	}

	assert.ElementsMatch(t, []string{
		"resolved_policy.40",
		"header.41",
		"9",
		"package_admission.7",
		"package_admission.cooldown.5",
		"field_sources.6",
	}, paths)

	assert.Equal(t, 72*time.Hour, p.Values["package_admission.cooldown.min_age"], "the known fields apply")
}

func TestOrderedEnumTakesTheStrictestValue(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().SetScanGate(scangatev1.ScanGate_builder{
		FailOn:        scangatev1.Severity(9).Enum(),
		FailOnAttacks: scangatev1.AttackGate(7).Enum(),
	}.Build())

	p := decode(t, policy)

	assert.Equal(t, Enum(scangatev1.Severity_SEVERITY_INFO), p.Values["scan_gate.fail_on"])
	assert.Equal(t, Enum(scangatev1.AttackGate_ATTACK_GATE_FAIL), p.Values["scan_gate.fail_on_attacks"])
	assert.Empty(t, p.FailedCapabilities)

	require.Len(t, p.Exceptions, 2)
	for _, e := range p.Exceptions {
		assert.Equal(t, ExceptionUnknownEnumValue, e.Kind)
	}
}

func TestKnownEnumValue(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().SetScanGate(scangatev1.ScanGate_builder{FailOn: scangatev1.Severity_SEVERITY_HIGH.Enum()}.Build())

	p := decode(t, policy)
	assert.Equal(t, Enum(scangatev1.Severity_SEVERITY_HIGH), p.Values["scan_gate.fail_on"])
	assert.Empty(t, p.Exceptions)
}

func TestInvalidValueFailsItsCapability(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().GetPackageAdmission().GetCooldown().SetMinAge(durationpb.New(2 * 365 * 24 * time.Hour))
	policy.GetSpec().SetScanGate(scangatev1.ScanGate_builder{FailOn: scangatev1.Severity_SEVERITY_HIGH.Enum()}.Build())

	p := decode(t, policy)

	assert.Equal(t, []string{"package_admission"}, p.FailedCapabilities)
	assert.NotContains(t, p.Values, "package_admission.cooldown.min_age")
	assert.Equal(t, Enum(scangatev1.Severity_SEVERITY_HIGH), p.Values["scan_gate.fail_on"], "the other capability applies")

	require.Len(t, p.Exceptions, 1)
	assert.Equal(t, ExceptionInvalidValue, p.Exceptions[0].Kind)
	assert.Equal(t, "package_admission.cooldown.min_age", p.Exceptions[0].Path)
}

func TestZeroEnumFailsItsCapability(t *testing.T) {
	policy := newPolicy()
	policy.GetSpec().SetScanGate(scangatev1.ScanGate_builder{FailOn: scangatev1.Severity_SEVERITY_UNSPECIFIED.Enum()}.Build())

	p := decode(t, policy)

	assert.Equal(t, []string{"scan_gate"}, p.FailedCapabilities)
	assert.NotContains(t, p.Values, "scan_gate.fail_on")
	assert.Equal(t, 72*time.Hour, p.Values["package_admission.cooldown.min_age"], "the other capability applies")

	require.Len(t, p.Exceptions, 1)
	assert.Equal(t, ExceptionInvalidValue, p.Exceptions[0].Kind)
	assert.Equal(t, "scan_gate.fail_on", p.Exceptions[0].Path)
}

func TestRestore(t *testing.T) {
	valid := decode(t, newPolicy())

	bad := newPolicy()
	bad.GetSpec().GetPackageAdmission().GetCooldown().SetMinAge(durationpb.New(-time.Hour))

	second := decode(t, bad)
	require.Equal(t, []string{"package_admission"}, second.FailedCapabilities)

	assert.Equal(t, []string{"package_admission"}, second.Restore(valid))
	assert.Equal(t, 72*time.Hour, second.Values["package_admission.cooldown.min_age"])
	assert.Equal(t, recordv1.Layer_LAYER_OVERRIDE, second.Sources[SourceKey{Path: "package_admission.cooldown.min_age"}].Layer)
	assert.Empty(t, second.Restore(valid), "a capability is restored once")

	// The capability fails in two policies in a row. The value of the first
	// valid policy survives.
	third := decode(t, bad)
	assert.Equal(t, []string{"package_admission"}, third.Restore(second))
	assert.Equal(t, 72*time.Hour, third.Values["package_admission.cooldown.min_age"])

	// A previous policy that failed the capability and did not restore it
	// has no value to give.
	fourth := decode(t, bad)
	assert.Empty(t, fourth.Restore(decode(t, bad)))
	assert.NotContains(t, fourth.Values, "package_admission.cooldown.min_age")

	assert.Empty(t, decode(t, bad).Restore(nil))
}

func TestFailedCapabilityHasNoSources(t *testing.T) {
	prev := newPolicy()
	prev.GetSpec().GetPackageAdmission().ClearCooldown()
	prev.SetFieldSources(nil)
	valid := decode(t, prev)
	require.NotContains(t, valid.Values, "package_admission.cooldown.min_age")

	bad := newPolicy()
	bad.GetSpec().GetPackageAdmission().GetCooldown().SetMinAge(durationpb.New(-time.Hour))

	next := decode(t, bad)
	require.Equal(t, []string{"package_admission"}, next.FailedCapabilities)
	key := SourceKey{Path: "package_admission.cooldown.min_age"}
	assert.NotContains(t, next.Sources, key, "a rejected value names no layer")

	next.Restore(valid)
	assert.NotContains(t, next.Values, "package_admission.cooldown.min_age")
	assert.NotContains(t, next.Sources, key, "the previous policy had no source for it")
}

func rule(id string, effect corev1.Effect, purl string, scopes ...corev1.TrustScope) *corev1.PackageRule {
	return corev1.PackageRule_builder{
		RuleId:    id,
		Selector:  corev1.PackageSelector_builder{Purl: purl}.Build(),
		Effect:    effect,
		Scopes:    scopes,
		Reason:    "IR-2291 credential stealer",
		ExpiresAt: timestamppb.New(expires),
	}.Build()
}

func withVers(r *corev1.PackageRule, vers string) *corev1.PackageRule {
	r.GetSelector().SetVers(vers)
	return r
}

func TestPackageRules(t *testing.T) {
	const (
		trust = corev1.Effect_EFFECT_TRUST
		block = corev1.Effect_EFFECT_BLOCK

		malware  = corev1.TrustScope_TRUST_SCOPE_MALWARE
		cooldown = corev1.TrustScope_TRUST_SCOPE_COOLDOWN
		vuln     = corev1.TrustScope_TRUST_SCOPE_VULNERABILITY
	)

	unknownInRule := func(r *corev1.PackageRule) *corev1.PackageRule {
		addUnknown(r, 9)
		return r
	}
	unknownInSelector := func(r *corev1.PackageRule) *corev1.PackageRule {
		addUnknown(r.GetSelector(), 9)
		return r
	}
	unknownInExpiry := func(r *corev1.PackageRule) *corev1.PackageRule {
		addUnknown(r.GetExpiresAt(), 9)
		return r
	}
	noExpiry := func(r *corev1.PackageRule) *corev1.PackageRule {
		r.ClearExpiresAt()
		return r
	}
	noReason := func(r *corev1.PackageRule) *corev1.PackageRule {
		r.SetReason("")
		return r
	}

	cases := []struct {
		name         string
		rule         *corev1.PackageRule
		want         *match.Rule
		wantDropped  bool
		wantDetail   string
		wantUnknown  string
		wantLimit    string
		wantLimitMsg string
	}{
		{
			name: "block entry",
			rule: rule("r_1", block, "pkg:npm/evil"),
			want: &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/evil", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
		},
		{
			name: "trust entry",
			rule: withVers(rule("r_1", trust, "pkg:npm/fp", cooldown, malware), "vers:npm/>=1.0.0|<2.0.0"),
			want: &match.Rule{
				ID: "r_1", Effect: match.EffectTrust, PURL: "pkg:npm/fp", Vers: "vers:npm/>=1.0.0|<2.0.0",
				Scopes: []match.Scope{match.ScopeMalware, match.ScopeCooldown}, Reason: "IR-2291 credential stealer",
				ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud},
			},
		},
		{
			name:        "trust entry with an unknown field",
			rule:        unknownInRule(rule("r_1", trust, "pkg:npm/fp", malware)),
			wantDropped: true,
		},
		{
			name:        "trust entry with an unknown field in its selector",
			rule:        unknownInSelector(rule("r_1", trust, "pkg:npm/fp", malware)),
			wantDropped: true,
		},
		{
			name:        "block entry with an unknown field",
			rule:        unknownInRule(rule("r_1", block, "pkg:npm/evil")),
			want:        &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/evil", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
			wantUnknown: "core.package_rules.9",
		},
		{
			name:        "trust entry with an unknown field in its expiry",
			rule:        unknownInExpiry(rule("r_1", trust, "pkg:npm/fp", malware)),
			wantDropped: true,
			wantDetail:  "core.package_rules.expires_at.9",
		},
		{
			name:        "trust entry with no expiry",
			rule:        noExpiry(rule("r_1", trust, "pkg:npm/fp", malware)),
			wantDropped: true,
			wantDetail:  "no expiry",
		},
		{
			name:        "trust entry with no reason",
			rule:        noReason(rule("r_1", trust, "pkg:npm/fp", malware)),
			wantDropped: true,
			wantDetail:  "contract limit",
		},
		{
			name:        "trust entry with a selector that is not a purl",
			rule:        rule("r_1", trust, "pkg:npm", malware),
			wantDropped: true,
			wantDetail:  "contract limit",
		},
		{
			name:         "block entry with no reason",
			rule:         noReason(rule("r_1", block, "pkg:npm/evil")),
			want:         &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/evil", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
			wantLimit:    "core.package_rules.reason",
			wantLimitMsg: "string.min_len",
		},
		{
			name:         "block entry with a scope",
			rule:         rule("r_1", block, "pkg:npm/evil", malware),
			want:         &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/evil", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
			wantLimit:    "core.package_rules",
			wantLimitMsg: "package_rule.block_has_no_scope",
		},
		{
			name:         "block entry with a version in the prefix",
			rule:         rule("r_1", block, "pkg:npm/x@1.*"),
			want:         &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/x@1.*", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
			wantLimit:    "core.package_rules.selector",
			wantLimitMsg: "package_selector.prefix_has_no_version",
		},
		{
			name:        "trust entry with only an unknown scope",
			rule:        rule("r_1", trust, "pkg:npm/fp", corev1.TrustScope(9)),
			wantDropped: true,
		},
		{
			name: "trust entry keeps its known scopes",
			rule: rule("r_1", trust, "pkg:npm/fp", corev1.TrustScope(9), malware),
			want: &match.Rule{
				ID: "r_1", Effect: match.EffectTrust, PURL: "pkg:npm/fp", Scopes: []match.Scope{match.ScopeMalware},
				Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud},
			},
		},
		{
			name: "trust entry with only the vulnerability scope stays",
			rule: rule("r_1", trust, "pkg:npm/fp", vuln),
			want: &match.Rule{
				ID: "r_1", Effect: match.EffectTrust, PURL: "pkg:npm/fp", Scopes: []match.Scope{match.ScopeVulnerability},
				Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud},
			},
		},
		{
			name:        "trust entry with no scope",
			rule:        rule("r_1", trust, "pkg:npm/fp"),
			wantDropped: true,
		},
		{
			name:        "unknown effect",
			rule:        rule("r_1", corev1.Effect(5), "pkg:npm/evil"),
			wantDropped: true,
		},
		{
			name:        "trust entry with a prefix",
			rule:        rule("r_1", trust, "pkg:npm/@acme/*", malware),
			wantDropped: true,
		},
		{
			name: "block entry with a prefix",
			rule: rule("r_1", block, "pkg:npm/@acme/*"),
			want: &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/@acme/*", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
		},
		{
			name:        "trust entry with no rule id",
			rule:        rule("", trust, "pkg:npm/fp", malware),
			wantDropped: true,
		},
		{
			name: "block entry with no rule id",
			rule: rule("", block, "pkg:npm/evil"),
			want: &match.Rule{ID: match.BlockRuleIDNone, Effect: match.EffectBlock, PURL: "pkg:npm/evil", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
		},
		{
			name:        "trust entry with a vers that does not parse",
			rule:        withVers(rule("r_1", trust, "pkg:npm/fp", malware), "vers:npm/>=1|>=2"),
			wantDropped: true,
		},
		{
			name: "block entry with a vers that does not parse",
			rule: withVers(rule("r_1", block, "pkg:npm/evil"), "vers:npm/>=1|>=2"),
			want: &match.Rule{ID: "r_1", Effect: match.EffectBlock, PURL: "pkg:npm/evil", Vers: "vers:npm/>=1|>=2", Reason: "IR-2291 credential stealer", ExpiresAt: expires, Origin: match.Origin{Kind: match.OriginCloud}},
		},
		{
			name: "expired trust entry stays",
			rule: func() *corev1.PackageRule {
				r := rule("r_1", trust, "pkg:npm/fp", malware)
				r.SetExpiresAt(timestamppb.New(issuedAt.Add(-time.Hour)))
				return r
			}(),
			want: &match.Rule{
				ID: "r_1", Effect: match.EffectTrust, PURL: "pkg:npm/fp", Scopes: []match.Scope{match.ScopeMalware},
				Reason: "IR-2291 credential stealer", ExpiresAt: issuedAt.Add(-time.Hour), Origin: match.Origin{Kind: match.OriginCloud},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := newPolicy()
			policy.GetSpec().SetCore(corev1.Core_builder{PackageRules: []*corev1.PackageRule{tc.rule}}.Build())

			p := decode(t, policy)

			if tc.wantDropped {
				assert.Empty(t, p.Rules)
				require.Len(t, p.Exceptions, 1)
				assert.Equal(t, ExceptionDroppedRule, p.Exceptions[0].Kind)
				assert.Equal(t, "core.package_rules", p.Exceptions[0].Path)
				assert.Equal(t, tc.rule.GetRuleId(), p.Exceptions[0].RuleID)
				assert.Contains(t, p.Exceptions[0].Detail, tc.wantDetail)
				return
			}

			require.NotNil(t, tc.want)
			require.Len(t, p.Rules, 1)
			assert.Equal(t, *tc.want, p.Rules[0])

			if tc.wantLimit != "" {
				require.Len(t, p.Exceptions, 1)
				assert.Equal(t, ExceptionInvalidValue, p.Exceptions[0].Kind)
				assert.Equal(t, tc.wantLimit, p.Exceptions[0].Path)
				assert.Equal(t, tc.rule.GetRuleId(), p.Exceptions[0].RuleID)
				assert.Contains(t, p.Exceptions[0].Detail, tc.wantLimitMsg)
				return
			}

			if tc.wantUnknown == "" {
				assert.Empty(t, p.Exceptions)
				return
			}

			require.Len(t, p.Exceptions, 1)
			assert.Equal(t, ExceptionUnknownField, p.Exceptions[0].Kind)
			assert.Equal(t, tc.wantUnknown, p.Exceptions[0].Path)
			assert.Equal(t, tc.rule.GetRuleId(), p.Exceptions[0].RuleID)
		})
	}
}

func TestFields(t *testing.T) {
	fields := Fields()

	byPath := map[string]Field{}
	var paths []string
	for _, f := range fields {
		byPath[f.Path] = f
		paths = append(paths, f.Path)
	}

	assert.IsIncreasing(t, paths)

	assert.Equal(t, Field{
		Path:               "core.package_rules",
		Capability:         "core",
		Merge:              optionsv1.Merge_MERGE_BY_KEY,
		DefaultEnforcement: optionsv1.Enforcement_ENFORCEMENT_MANDATORY,
		BindingRequired:    true,
	}, byPath["core.package_rules"])

	assert.Equal(t, Field{
		Path:               "package_admission.cooldown.min_age",
		Capability:         "package_admission",
		Merge:              optionsv1.Merge_MERGE_REPLACE,
		DefaultEnforcement: optionsv1.Enforcement_ENFORCEMENT_MANDATORY,
		Stricter:           optionsv1.Stricter_STRICTER_HIGHER,
	}, byPath["package_admission.cooldown.min_age"])

	assert.Contains(t, byPath, "scan_gate.fail_on")
	assert.Contains(t, byPath, "scan_gate.fail_on_attacks")
}

func TestExceptionFieldException(t *testing.T) {
	cases := []struct {
		kind ExceptionKind
		want recordv1.FieldExceptionReason
	}{
		{ExceptionUnknownField, recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNKNOWN_FIELD},
		{ExceptionUnknownEnumValue, recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNKNOWN_ENUM_VALUE},
		{ExceptionDroppedRule, recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_DROPPED_RULE},
		{ExceptionUnsupported, recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNSUPPORTED},
		{ExceptionInvalidValue, recordv1.FieldExceptionReason_FIELD_EXCEPTION_REASON_UNSPECIFIED},
	}

	for _, tc := range cases {
		t.Run(tc.want.String(), func(t *testing.T) {
			fe := Exception{Path: "core.package_rules", Kind: tc.kind, RuleID: "r_a41", Detail: "unknown scope"}.FieldException()
			assert.Equal(t, "core.package_rules", fe.GetPath())
			assert.Equal(t, tc.want, fe.GetReason())
			assert.Equal(t, "rule r_a41: unknown scope", fe.GetDetail())
		})
	}
}
