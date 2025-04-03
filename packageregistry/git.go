package packageregistry

import (
	"fmt"
	"strings"

	giturls "github.com/whilp/git-urls"
)

// getNormalizedGitURL normalizes different Git URL formats to a standardized form
// Supports self-hosted Git repositories as well as popular services
func getNormalizedGitURL(inputURL string) (string, error) {
	//Check if its even a git URL
	if inputURL == "" {
		return "", nil
	}

	inputURL = strings.TrimPrefix(inputURL, "git+")

	// Special handling for SCP-style SSH URLs with port numbers
	// e.g. git@host:port/path.git -> ssh://git@host:port/path.git
	if strings.HasPrefix(inputURL, "git@") && strings.Contains(inputURL, ":") {
		parts := strings.SplitN(inputURL, ":", 2)
		if len(parts) == 2 {
			host := parts[0][4:] // Remove "git@" prefix
			path := parts[1]

			// Check if the first part of the path is a port number
			pathParts := strings.SplitN(path, "/", 2)
			if len(pathParts) == 2 {
				if _, err := fmt.Sscanf(pathParts[0], "%d", new(int)); err == nil {
					// If it's a port number, reconstruct as SSH URL
					inputURL = fmt.Sprintf("ssh://git@%s:%s/%s", host, pathParts[0], pathParts[1])
				}
			}
		}
	}

	inputURL = strings.TrimSuffix(inputURL, ".git")

	// Parse the Git URL using the git-urls library
	u, err := giturls.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse git URL: %w", err)
	}

	// Always use HTTPS for the normalized form
	u.Scheme = "https"

	// Remove username and password if present
	u.User = nil

	return u.String(), nil
}
