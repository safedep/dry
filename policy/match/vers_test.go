package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseVers(t *testing.T) {
	cases := []struct {
		name    string
		vers    string
		want    string
		wantErr string
	}{
		{name: "range", vers: "vers:npm/>=1.2.0|<2.0.0", want: "vers:npm/>=1.2.0|<2.0.0"},
		{name: "all", vers: "vers:npm/*", want: "vers:npm/*"},
		{name: "one version", vers: "vers:pypi/1.0.3", want: "vers:pypi/=1.0.3"},
		{name: "white space", vers: " vers:npm/ >= 1.2.0 | < 2.0.0 ", want: "vers:npm/>=1.2.0|<2.0.0"},
		{name: "upper case scheme", vers: "vers:NPM/1.0.0", want: "vers:npm/=1.0.0"},
		{name: "not equal", vers: "vers:npm/!=1.0.0", want: "vers:npm/!=1.0.0"},
		{name: "points and bounds", vers: "vers:npm/>=1.0.0|!=1.5.0|<2.0.0|=3.0.0", want: "vers:npm/!=1.5.0|=3.0.0|>=1.0.0|<2.0.0"},
		{name: "two intervals", vers: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0", want: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0"},
		{name: "encoded version", vers: "vers:npm/=1.0.0%7Cx", want: "vers:npm/=1.0.0%7Cx"},
		{name: "no scheme prefix", vers: "npm/>=1.0.0", wantErr: "does not start"},
		{name: "no versioning scheme", vers: "vers:>=1.0.0", wantErr: "no versioning scheme"},
		{name: "empty versioning scheme", vers: "vers:/>=1.0.0", wantErr: "invalid versioning scheme"},
		{name: "bad versioning scheme", vers: "vers:n_pm/>=1.0.0", wantErr: "invalid versioning scheme"},
		{name: "empty list", vers: "vers:npm/", wantErr: "empty constraint"},
		{name: "empty constraint", vers: "vers:npm/>=1.0.0||<2.0.0", wantErr: "empty constraint"},
		{name: "trailing separator", vers: "vers:npm/>=1.0.0|", wantErr: "empty constraint"},
		{name: "star with others", vers: "vers:npm/*|>=1.0.0", wantErr: `"*" must be the only`},
		{name: "no version", vers: "vers:npm/>=", wantErr: "no version"},
		{name: "double comparator", vers: "vers:npm/>>1.0.0", wantErr: "invalid comparator"},
		{name: "bad escape", vers: "vers:npm/=1.0.0%zz", wantErr: "invalid URL escape"},
		{name: "same version twice", vers: "vers:npm/>=1.0.0|<1.0.0", wantErr: "twice"},
		{name: "two lower bounds", vers: "vers:npm/>=1.0.0|>=2.0.0", wantErr: "alternate"},
		{name: "two upper bounds", vers: "vers:npm/<1.0.0|<=2.0.0", wantErr: "alternate"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVers(tc.vers)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestRangeContains(t *testing.T) {
	cases := []struct {
		name    string
		vers    string
		version string
		want    bool
		wantErr bool
	}{
		{name: "all", vers: "vers:npm/*", version: "0.0.1", want: true},
		{name: "equal", vers: "vers:npm/1.0.0", version: "1.0.0", want: true},
		{name: "equal by order", vers: "vers:npm/1.0", version: "1.0.0", want: true},
		{name: "not equal", vers: "vers:npm/1.0.0", version: "1.0.1"},
		{name: "not equal comparator hit", vers: "vers:npm/!=1.0.0", version: "1.0.0"},
		{name: "not equal comparator miss", vers: "vers:npm/!=1.0.0", version: "1.0.1", want: true},
		{name: "two equal", vers: "vers:npm/1.0.0|2.0.0", version: "2.0.0", want: true},
		{name: "less", vers: "vers:npm/<2.0.0", version: "1.9.9", want: true},
		{name: "less at bound", vers: "vers:npm/<2.0.0", version: "2.0.0"},
		{name: "less or equal at bound", vers: "vers:npm/<=2.0.0", version: "2.0.0", want: true},
		{name: "greater", vers: "vers:npm/>2.0.0", version: "2.0.1", want: true},
		{name: "greater at bound", vers: "vers:npm/>2.0.0", version: "2.0.0"},
		{name: "greater or equal at bound", vers: "vers:npm/>=2.0.0", version: "2.0.0", want: true},
		{name: "interval low bound", vers: "vers:npm/>=1.2.0|<2.0.0", version: "1.2.0", want: true},
		{name: "interval inside", vers: "vers:npm/>=1.2.0|<2.0.0", version: "1.5.0", want: true},
		{name: "interval high bound", vers: "vers:npm/>=1.2.0|<2.0.0", version: "2.0.0"},
		{name: "interval below", vers: "vers:npm/>=1.2.0|<2.0.0", version: "1.1.9"},
		{name: "interval with a hole", vers: "vers:npm/>=1.0.0|!=1.5.0|<2.0.0", version: "1.5.0"},
		{name: "interval with a point outside", vers: "vers:npm/>=1.0.0|<2.0.0|=3.0.0", version: "3.0.0", want: true},
		{name: "two intervals first", vers: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0", version: "1.5.0", want: true},
		{name: "two intervals gap", vers: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0", version: "2.5.0"},
		{name: "two intervals second", vers: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0", version: "3.5.0", want: true},
		{name: "two intervals above", vers: "vers:npm/>=1.0.0|<2.0.0|>=3.0.0|<4.0.0", version: "4.0.0"},
		{name: "open ends low", vers: "vers:npm/<1.0.0|>=2.0.0", version: "0.5.0", want: true},
		{name: "open ends middle", vers: "vers:npm/<1.0.0|>=2.0.0", version: "1.5.0"},
		{name: "open ends high", vers: "vers:npm/<1.0.0|>=2.0.0", version: "2.0.0", want: true},
		{name: "bounds out of order", vers: "vers:npm/<2.0.0|>=1.0.0", version: "1.5.0", wantErr: true},
		{name: "version does not compare", vers: "vers:npm/>=1.0.0", version: "beta", wantErr: true},
		{name: "no version", vers: "vers:npm/>=1.0.0", version: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rng, err := ParseVers(tc.vers)
			require.NoError(t, err)

			got, err := rng.Contains(testPackage{key: "npm/x", version: tc.version})
			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// FuzzParseVers checks that ParseVers never panics, that a parsed range
// prints a form that parses back to the same range, and that Contains never
// panics.
func FuzzParseVers(f *testing.F) {
	for _, seed := range []string{
		"vers:npm/>=1.2.0|<2.0.0",
		"vers:npm/*",
		"vers:pypi/1.0.3",
		"vers:npm/!=1.0.0|>=1.0.0|<2.0.0",
		"vers:npm/<1|>=2|<3|>=4",
		"vers:maven/=1.0%7C2",
		"vers:npm/>=",
		"vers:/",
		"vers:npm/||",
	} {
		f.Add(seed, "1.5.0")
	}

	f.Fuzz(func(t *testing.T, vers, version string) {
		rng, err := ParseVers(vers)
		if err != nil {
			return
		}

		again, err := ParseVers(rng.String())
		require.NoError(t, err, "parse %q", rng.String())
		assert.Equal(t, rng, again)

		_, _ = rng.Contains(testPackage{key: "npm/x", version: version})
	})
}
