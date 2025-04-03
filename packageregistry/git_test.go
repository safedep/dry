package packageregistry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNormalizedGitURL(t *testing.T) {
	cases := []struct {
		name     string
		gitURL   string
		expected string
	}{
		{
			name:     "normal https url",
			gitURL:   "https://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "normal git url",
			gitURL:   "git://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "normal ssh url",
			gitURL:   "ssh://git@github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "normal git url with git @",
			gitURL:   "git@github.com:expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "custom git server",
			gitURL:   "git@github.com:kubernetes/kubernetes.git",
			expected: "https://github.com/kubernetes/kubernetes",
		},
		{
			name:     "git protocoal url",
			gitURL:   "git://gitlab.com/gitlab-org/gitlab.git",
			expected: "https://gitlab.com/gitlab-org/gitlab",
		},
		{
			name:     "http without .git suffix",
			gitURL:   "https://github.com/user/repo",
			expected: "https://github.com/user/repo",
		},
		{
			name:     "bitbucket url",
			gitURL:   "http://bitbucket.org/user/project.git",
			expected: "https://bitbucket.org/user/project",
		},
		{
			name:     "ssh protocol git url with custom host and port",
			gitURL:   "ssh://git@git.example.com:2222/user/project.git",
			expected: "https://git.example.com:2222/user/project",
		},
		{
			name:     "internal git server",
			gitURL:   "https://git.internal.company.com/team/project",
			expected: "https://git.internal.company.com/team/project",
		},
		{
			name:     "npm style https url",
			gitURL:   "git+https://github.com/expressjs/express.git",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "npm style ssh url without .git suffix",
			gitURL:   "git+ssh://git@github.com/expressjs/express",
			expected: "https://github.com/expressjs/express",
		},
		{
			name:     "custom git server with port",
			gitURL:   "git@mypersonalserver.com:5454/something.git",
			expected: "https://mypersonalserver.com:5454/something",
		},
		{
			name:     "empty url",
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
