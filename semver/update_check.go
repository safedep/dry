package semver

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	mver "github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v74/github"
)

// UpdateCheckResult contains the result of a version update check.
type UpdateCheckResult struct {
	// IsLatest is true if the current version is the latest release.
	IsLatest bool

	// UpdateMessage is a human-readable message suggesting an update
	// when the current version is not the latest. Empty if already latest.
	UpdateMessage string
}

// parseGitHubURL extracts the owner and repo name from a GitHub repository URL.
// Supported formats:
//   - https://github.com/owner/repo
//   - https://github.com/owner/repo.git
//   - github.com/owner/repo
func parseGitHubURL(githubURL string) (owner, repo string, err error) {
	// Add scheme if missing so url.Parse works correctly
	if !strings.Contains(githubURL, "://") {
		githubURL = "https://" + githubURL
	}

	u, err := url.Parse(githubURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	if u.Host != "github.com" && u.Host != "www.github.com" {
		return "", "", fmt.Errorf("not a GitHub URL: %s", u.Host)
	}

	// Trim leading/trailing slashes and split the path
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository URL: expected owner/repo in path")
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid GitHub repository URL: owner or repo is empty")
	}

	return owner, repo, nil
}

// CheckUpdate checks whether the given current version is the latest release
// for the GitHub repository identified by the given URL. It uses the GitHub
// Releases API to fetch the latest release and compares it using semver.
//
// The githubURL should point to a GitHub repository (e.g., "https://github.com/owner/repo").
// The currentVersion should be a valid semver string (e.g., "v1.2.3" or "1.2.3").
func CheckUpdate(ctx context.Context, githubURL, currentVersion string) (*UpdateCheckResult, error) {
	owner, repo, err := parseGitHubURL(githubURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse GitHub URL: %w", err)
	}

	current, err := mver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version %q: %w", currentVersion, err)
	}

	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release for %s/%s: %w", owner, repo, err)
	}

	latestTag := release.GetTagName()
	if latestTag == "" {
		return nil, fmt.Errorf("latest release for %s/%s has no tag", owner, repo)
	}

	latest, err := mver.NewVersion(latestTag)
	if err != nil {
		return nil, fmt.Errorf("latest release tag %q is not a valid semver: %w", latestTag, err)
	}

	result := &UpdateCheckResult{}

	if current.LessThan(latest) {
		repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
		result.UpdateMessage = fmt.Sprintf("Update available: %s -> %s. Visit %s/releases/latest to update.",
			current.Original(), latest.Original(), repoURL)
	} else {
		result.IsLatest = true
	}

	return result, nil
}
