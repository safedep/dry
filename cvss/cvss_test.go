package cvss

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseStringParsing(t *testing.T) {
	cases := []struct {
		name    string
		version CvssVersion
		base    string
		err     error
	}{
		{
			name:    "valid v2",
			version: CVSS_V2,
			base:    "AV:N/AC:L/Au:N/C:C/I:C/A:C",
			err:     nil,
		},
		{
			name:    "valid v3",
			version: CVSS_V3,
			base:    "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
			err:     nil,
		},
		{
			name:    "valid v4",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:N/VI:N/VA:N/SC:H/SI:N/SA:N",
			err:     nil,
		},
		{
			name:    "invalid v2",
			version: CVSS_V2,
			base:    "<invalid>",
			err:     errors.New("invalid vector"),
		},
		{
			name:    "invalid v3",
			version: CVSS_V3,
			base:    "<invalid>",
			err:     errors.New("invalid vector"),
		},
		{
			name:    "invalid v4",
			version: CVSS_V4,
			base:    "<invalid>",
			err:     errors.New("invalid vector"),
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCvssBaseString(test.base, test.version)

			if test.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.err.Error())
			}
		})
	}
}

func TestBaseSeverity(t *testing.T) {
	cases := []struct {
		name    string
		version CvssVersion
		base    string
		risk    CvssRisk
	}{
		{
			name:    "v2 high",
			version: CVSS_V2,
			base:    "AV:N/AC:L/Au:N/C:C/I:C/A:C",
			risk:    HIGH,
		},
		{
			name:    "v2 medium",
			version: CVSS_V2,
			base:    "AV:N/AC:H/Au:S/C:C/I:N/A:N",
			risk:    MEDIUM,
		},
		{
			name:    "v2 low",
			version: CVSS_V2,
			base:    "AV:N/AC:L/Au:N/C:N/I:N/A:N",
			risk:    LOW,
		},
		{
			name:    "v3 critical",
			version: CVSS_V3,
			base:    "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
			risk:    CRITICAL,
		},
		{
			name:    "v3 high",
			version: CVSS_V3,
			base:    "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:H",
			risk:    HIGH,
		},
		{
			name:    "v3 medium",
			version: CVSS_V3,
			base:    "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:L/I:L/A:N",
			risk:    MEDIUM,
		},
		{
			name:    "v3 low",
			version: CVSS_V3,
			base:    "CVSS:3.1/AV:N/AC:H/PR:N/UI:N/S:U/C:N/I:L/A:N",
			risk:    LOW,
		},
		{
			name:    "v3 none",
			version: CVSS_V3,
			base:    "CVSS:3.0/AV:N/AC:L/PR:N/UI:N/S:U/C:N/I:N/A:N",
			risk:    NONE,
		},
		{
			name:    "v4 critical",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:L/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:H/SI:H/SA:H",
			risk:    CRITICAL,
		},
		{
			name:    "v4 high",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:N/VI:N/VA:H/SC:N/SI:N/SA:N",
			risk:    HIGH,
		},
		{
			name:    "v4 medium",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:N/AC:L/AT:N/PR:L/UI:N/VC:N/VI:N/VA:N/SC:L/SI:L/SA:N/MAV:A",
			risk:    MEDIUM,
		},
		{
			name:    "v4 low",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:L/AC:L/AT:N/PR:N/UI:N/VC:L/VI:L/VA:L/SC:N/SI:N/SA:N/E:U",
			risk:    LOW,
		},
		{
			name:    "v4 none",
			version: CVSS_V4,
			base:    "CVSS:4.0/AV:A/AC:L/AT:P/PR:L/UI:P/VC:N/VI:N/VA:N/SC:N/SI:N/SA:N",
			risk:    NONE,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			c, err := NewCvssBaseString(test.base, test.version)
			assert.NoError(t, err)

			assert.Equal(t, test.risk, c.Severity())
		})
	}

}
