package project

import (
	"fmt"
	"testing"

	corev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/core/v1"
	"google.golang.org/protobuf/proto"
)

// BenchmarkDecode measures Decode of a resolved policy with 10,000 package
// rules: blocks, prefix blocks and trust entries with a vers range. pmg runs
// Decode before each command, and Decode checks the contract limits of each
// rule with protovalidate.
func BenchmarkDecode(b *testing.B) {
	rules := make([]*corev1.PackageRule, 0, 10_000)
	for i := range 10_000 {
		id := fmt.Sprintf("r_%08d", i)
		switch i % 4 {
		case 0:
			rules = append(rules, rule(id, corev1.Effect_EFFECT_BLOCK, fmt.Sprintf("pkg:npm/evil-%d", i)))
		case 1:
			rules = append(rules, rule(id, corev1.Effect_EFFECT_BLOCK, fmt.Sprintf("pkg:npm/@scope%d/*", i)))
		case 2:
			rules = append(rules, withVers(
				rule(id, corev1.Effect_EFFECT_TRUST, fmt.Sprintf("pkg:pypi/fp-%d", i), corev1.TrustScope_TRUST_SCOPE_MALWARE),
				"vers:pypi/>=1.0|<2.0"))
		default:
			rules = append(rules, rule(id, corev1.Effect_EFFECT_TRUST, fmt.Sprintf("pkg:npm/fp-%d@1.0.0", i),
				corev1.TrustScope_TRUST_SCOPE_COOLDOWN))
		}
	}

	policy := newPolicy()
	policy.GetSpec().SetCore(corev1.Core_builder{PackageRules: rules}.Build())
	raw, err := proto.Marshal(policy)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	for b.Loop() {
		p, err := Decode(raw, endpointExpect)
		if err != nil {
			b.Fatal(err)
		}
		if len(p.Rules) != len(rules) {
			b.Fatalf("decoded %d rules, want %d", len(p.Rules), len(rules))
		}
	}
}
