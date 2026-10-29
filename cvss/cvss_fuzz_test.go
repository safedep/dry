package cvss

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cvssV3Metric is one metric of the v3.1 specification with every value it
// can take. The order of v3Base and v3Optional is the order of the spec.
type cvssV3Metric struct {
	name   string
	values []string
}

var v3Base = []cvssV3Metric{
	{"AV", []string{"N", "A", "L", "P"}},
	{"AC", []string{"L", "H"}},
	{"PR", []string{"N", "L", "H"}},
	{"UI", []string{"N", "R"}},
	{"S", []string{"U", "C"}},
	{"C", []string{"H", "L", "N"}},
	{"I", []string{"H", "L", "N"}},
	{"A", []string{"H", "L", "N"}},
}

var v3Optional = []cvssV3Metric{
	{"E", []string{"X", "U", "P", "F", "H"}},
	{"RL", []string{"X", "O", "T", "W", "U"}},
	{"RC", []string{"X", "U", "R", "C"}},
	{"CR", []string{"X", "L", "M", "H"}},
	{"IR", []string{"X", "L", "M", "H"}},
	{"AR", []string{"X", "L", "M", "H"}},
	{"MAV", []string{"X", "N", "A", "L", "P"}},
	{"MAC", []string{"X", "L", "H"}},
	{"MPR", []string{"X", "N", "L", "H"}},
	{"MUI", []string{"X", "N", "R"}},
	{"MS", []string{"X", "U", "C"}},
	{"MC", []string{"X", "N", "L", "H"}},
	{"MI", []string{"X", "N", "L", "H"}},
	{"MA", []string{"X", "N", "L", "H"}},
}

// fuzzChoices turns the fuzzer's bytes into choices. When the bytes run out,
// each choice is 0, so every input gives a valid vector.
type fuzzChoices []byte

func (c *fuzzChoices) next(n int) int {
	if len(*c) == 0 {
		return 0
	}

	b := (*c)[0]
	*c = (*c)[1:]

	return int(b) % n
}

// v3Vectors returns a valid v3 vector with a random set of optional
// metrics, and the same vector with the base metrics only.
func v3Vectors(choices fuzzChoices) (string, string) {
	prefix := []string{"CVSS:3.0", "CVSS:3.1"}[choices.next(2)]

	base := []string{prefix}
	for _, m := range v3Base {
		base = append(base, m.name+":"+m.values[choices.next(len(m.values))])
	}

	full := append([]string{}, base...)
	for _, m := range v3Optional {
		if choices.next(2) == 1 {
			full = append(full, m.name+":"+m.values[choices.next(len(m.values))])
		}
	}

	return strings.Join(full, "/"), strings.Join(base, "/")
}

func FuzzV3SeverityComesFromBaseMetrics(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 1, 0, 0, 0, 0, 0, 1, 4, 1, 1, 1, 3})
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		vector, base := v3Vectors(data)

		want, err := NewCvssBaseString(base, CVSS_V3)
		require.NoError(t, err, base)

		got, err := NewCvssBaseString(vector, CVSS_V3)
		require.NoError(t, err, vector)

		assert.Equal(t, want.Severity(), got.Severity(), vector)
	})
}

func FuzzTrailingSlashDoesNotChangeResult(f *testing.F) {
	f.Add("CVSS:3.1/AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H/E:H", string(CVSS_V3))
	f.Add("CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:N/VA:N/SC:H/SI:N/SA:N", string(CVSS_V4))
	f.Add("AV:N/AC:L/Au:N/C:C/I:C/A:C", string(CVSS_V2))
	f.Add("<invalid>", string(CVSS_V3))

	f.Fuzz(func(t *testing.T, raw, version string) {
		if strings.HasSuffix(raw, "/") {
			t.Skip("the property holds for a vector with no trailing slash")
		}

		want, wantErr := NewCvssBaseString(raw, CvssVersion(version))
		got, gotErr := NewCvssBaseString(raw+"/", CvssVersion(version))

		require.Equal(t, wantErr == nil, gotErr == nil, raw)
		if wantErr != nil {
			return
		}

		assert.Equal(t, want.Severity(), got.Severity(), raw)
	})
}
