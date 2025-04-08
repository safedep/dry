package packageregistry

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// getNormalizedGitURL normalizes different Git URL formats to a standardized form
// Supports self-hosted Git repositories as well as popular services
func getNormalizedGitURL(inputURL string) (string, error) {
	if inputURL == "" {
		return "", nil
	}

	// Remove git+ prefix and .git suffix
	inputURL = strings.TrimPrefix(inputURL, "git+")
	inputURL = strings.TrimSuffix(inputURL, ".git")

	// Remove /tree/{version} from GitHub URLs
	if strings.Contains(inputURL, "/tree/") {
		parts := strings.Split(inputURL, "/tree/")
		if len(parts) > 1 {
			inputURL = parts[0]
		}
	}

	// Handle SCP-style SSH URLs (e.g., git@github.com:user/repo)
	if strings.HasPrefix(inputURL, "git@") {
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

	// Convert git:// URLs to https://
	if strings.HasPrefix(inputURL, "git://") {
		inputURL = "https://" + inputURL[6:]
	}

	// Convert ssh:// URLs to https://
	sshPattern := regexp.MustCompile(`^ssh://(?:git@)?(.+)$`)
	if matches := sshPattern.FindStringSubmatch(inputURL); matches != nil {
		inputURL = "https://" + matches[1]
	}

	// Handle SCP-style URLs without explicit protocol
	if strings.Contains(inputURL, ":") && !strings.Contains(inputURL, "://") {
		parts := strings.SplitN(inputURL, ":", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], "/") {
			inputURL = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
		}
	}

	// Parse the URL to validate and normalize it
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse git URL: %w", err)
	}

	// Convert scheme to https
	parsedURL.Scheme = "https"

	// Remove username and password from URL
	parsedURL.User = nil

	// Clean up the path
	path := parsedURL.Path
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	return fmt.Sprintf("https://%s/%s", parsedURL.Host, path), nil
}
