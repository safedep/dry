package packageregistry

import (
	"strings"

	giturl "github.com/kubescape/go-git-url"
)

// getNormalizedGitURL normalizes the git URL
func getNormalizedGitURL(gitURL string) (string, error) {
	//Check if its even a git URL
	if gitURL == "" {
		return "", nil
	}

	// go-git-url doesn't support git+ prefix, else all good
	gitURL = strings.TrimPrefix(gitURL, "git+")

	url, err := giturl.NewGitURL(gitURL)
	if err != nil {
		return "", err
	}
	return url.GetURL().String(), nil
}
