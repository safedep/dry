package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	defaultTimeout = 5 * time.Second
	defaultBaseURL = "https://api.github.com"
)

// Config holds the project-specific configuration for the update checker.
type Config struct {
	// Owner is the GitHub repository owner (e.g., "safedep").
	Owner string

	// Repo is the GitHub repository name (e.g., "vet").
	Repo string
}

// UpdateResult contains the result of a version update check.
type UpdateResult struct {
	UpdateAvailable bool   `json:"update_available"`
	LatestVersion   string `json:"latest_version"`
	CurrentVersion  string `json:"current_version"`
	ReleaseURL      string `json:"release_url"`
}

// Option configures the Checker.
type Option func(*Checker)

// Checker checks for newer versions of a project using GitHub releases.
type Checker struct {
	config  Config
	timeout time.Duration
	baseURL string
	client  *http.Client
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Checker) {
		c.client = client
	}
}

func WithBaseURL(baseURL string) Option {
	return func(c *Checker) {
		c.baseURL = baseURL
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Checker) {
		c.timeout = timeout
	}
}

// NewChecker creates a new update checker for the given project config.
func NewChecker(config Config, opts ...Option) *Checker {
	c := &Checker{
		config:  config,
		timeout: defaultTimeout,
		baseURL: defaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.client == nil {
		c.client = &http.Client{Timeout: c.timeout}
	}
	return c
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// Check synchronously checks if a newer version is available.
func (c *Checker) Check(ctx context.Context, currentVersion string) (*UpdateResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest",
		strings.TrimRight(c.baseURL, "/"), c.config.Owner, c.config.Repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	result := &UpdateResult{
		LatestVersion:  release.TagName,
		CurrentVersion: currentVersion,
		ReleaseURL:     release.HTMLURL,
	}

	if isNewer(currentVersion, release.TagName) {
		result.UpdateAvailable = true
	}

	return result, nil
}

// CheckAsync performs a non-blocking update check. The result is sent on
// the returned channel. If the check fails, the channel is closed without
// sending a value, so the caller is never blocked.
func (c *Checker) CheckAsync(ctx context.Context, currentVersion string) <-chan *UpdateResult {
	ch := make(chan *UpdateResult, 1)
	go func() {
		defer close(ch)
		result, err := c.Check(ctx, currentVersion)
		if err != nil {
			return
		}
		ch <- result
	}()
	return ch
}

func isNewer(current, latest string) bool {
	current = normalize(current)
	latest = normalize(latest)

	if !semver.IsValid(current) || !semver.IsValid(latest) {
		return false
	}

	return semver.Compare(latest, current) > 0
}

func normalize(v string) string {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}
