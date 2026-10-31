package match

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// benchRules returns n rules: block entries with and without a range, trust
// entries with a range, and prefix block entries of many lengths.
func benchRules(n int) []Rule {
	rules := make([]Rule, 0, n)
	for i := range n {
		id := fmt.Sprintf("r_%07d", i)
		switch i % 10 {
		case 0:
			rules = append(rules, block(id, fmt.Sprintf("pkg:npm/@scope%d/*", i)))
		case 1, 2, 3:
			rules = append(rules, withVers(trust(id, fmt.Sprintf("pkg:npm/pkg%d", i), ScopeMalware), "vers:npm/>=1.0.0|<2.0.0"))
		case 4, 5:
			rules = append(rules, withVers(block(id, fmt.Sprintf("pkg:npm/pkg%d", i)), "vers:npm/>=3.0.0|<4.0.0|>=5.0.0"))
		default:
			rules = append(rules, block(id, fmt.Sprintf("pkg:npm/pkg%d", i)))
		}
	}

	return rules
}

// BenchmarkDecide measures one check at 10,000 rules. The target is under
// 10 µs for each check. A linear scan took 79 µs in a proof of concept.
func BenchmarkDecide(b *testing.B) {
	ix, dropped := NewIndex(benchRules(10_000), testIdentity{})
	require.Empty(b, dropped)

	queries := map[string]Query{
		"trust hit":  {Package: mustPackage("pkg:npm/pkg5001@1.5.0"), Control: ScopeMalware, Verdict: VerdictMalware, Now: testNow},
		"block hit":  {Package: mustPackage("pkg:npm/pkg5006@1.5.0"), Control: ScopeMalware, Verdict: VerdictMalware, Now: testNow},
		"prefix hit": {Package: mustPackage("pkg:npm/@scope5000/lib@1.5.0"), Control: ScopeMalware, Now: testNow},
		"miss":       {Package: mustPackage("pkg:npm/clean@1.5.0"), Control: ScopeMalware, Now: testNow},
	}

	for name, q := range queries {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = ix.Decide(q)
			}
		})
	}
}

func BenchmarkNewIndex(b *testing.B) {
	rules := benchRules(10_000)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = NewIndex(rules, testIdentity{})
	}
}
