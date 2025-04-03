package packageregistry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNormalizedGitURL(t *testing.T) {
	cases := []struct {
		gitURL   string
		expected string
	}{
		{
			gitURL:   "git+https://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "git+ssh://git@github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "https://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "http://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "git://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "ssh://git@github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "git@github.com:expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "git@gitlab.com:expressjs/express.git",
			expected: "https://gitlab.com/expressjs/express",
		},
		{
			gitURL:   "https://github.com/expressjs/express",
			expected: "https://github.com/expressjs/express",
		},
		{
			gitURL:   "",
			expected: "",
		},
	}

	for _, c := range cases {
		t.Run(c.gitURL, func(t *testing.T) {
			t.Parallel()

			actual, err := getNormalizedGitURL(c.gitURL)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
