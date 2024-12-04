package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSemverConstraintResolver(t *testing.T) {
	cases := []struct {
		name      string
		constaint string
		lowest    string
		err       error
	}{
		{
			name:      "version is version",
			constaint: "1.2.3",
			lowest:    "1.2.3",
		},
		{
			name:      "version is greater than given",
			constaint: ">1.2.3",
			lowest:    "1.2.4",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			// Need fixing
			t.Skip()

			c, err := NewConstraintResolver(test.constaint)
			assert.NoError(t, err)

			lowest, err := c.Lowest()
			if test.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.err.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.lowest, lowest)
			}
		})
	}
}
