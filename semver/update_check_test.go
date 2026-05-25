package semver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			"standard HTTPS URL",
			"https://github.com/safedep/vet",
			"safedep", "vet", false,
		},
		{
			"URL with .git suffix",
			"https://github.com/safedep/vet.git",
			"safedep", "vet", false,
		},
		{
			"URL without scheme",
			"github.com/safedep/vet",
			"safedep", "vet", false,
		},
		{
			"URL with trailing slash",
			"https://github.com/safedep/vet/",
			"safedep", "vet", false,
		},
		{
			"URL with extra path segments",
			"https://github.com/safedep/vet/tree/main",
			"safedep", "vet", false,
		},
		{
			"www prefix",
			"https://www.github.com/safedep/vet",
			"safedep", "vet", false,
		},
		{
			"non-GitHub URL",
			"https://gitlab.com/owner/repo",
			"", "", true,
		},
		{
			"missing repo in path",
			"https://github.com/safedep",
			"", "", true,
		},
		{
			"empty path",
			"https://github.com/",
			"", "", true,
		},
		{
			"empty owner with repo",
			"https://github.com//repo",
			"", "", true,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			owner, repo, err := parseGitHubURL(test.url)
			if test.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.wantOwner, owner)
			assert.Equal(t, test.wantRepo, repo)
		})
	}
}
