package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T, tagName, htmlURL string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))

		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			json.NewEncoder(w).Encode(githubRelease{
				TagName: tagName,
				HTMLURL: htmlURL,
			})
		}
	}))
}

func TestCheck_UpdateAvailable(t *testing.T) {
	server := newTestServer(t, "v2.0.0", "https://github.com/safedep/vet/releases/tag/v2.0.0", http.StatusOK)
	defer server.Close()

	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	result, err := checker.Check(context.Background(), "v1.0.0")
	require.NoError(t, err)
	assert.True(t, result.UpdateAvailable)
	assert.Equal(t, "v2.0.0", result.LatestVersion)
	assert.Equal(t, "v1.0.0", result.CurrentVersion)
	assert.Equal(t, "https://github.com/safedep/vet/releases/tag/v2.0.0", result.ReleaseURL)
}

func TestCheck_AlreadyLatest(t *testing.T) {
	server := newTestServer(t, "v1.0.0", "https://github.com/safedep/vet/releases/tag/v1.0.0", http.StatusOK)
	defer server.Close()

	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	result, err := checker.Check(context.Background(), "v1.0.0")
	require.NoError(t, err)
	assert.False(t, result.UpdateAvailable)
	assert.Equal(t, "v1.0.0", result.LatestVersion)
}

func TestCheck_APIError(t *testing.T) {
	server := newTestServer(t, "", "", http.StatusInternalServerError)
	defer server.Close()

	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	result, err := checker.Check(context.Background(), "v1.0.0")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "500")
}

func TestCheckAsync_UpdateAvailable(t *testing.T) {
	server := newTestServer(t, "v2.0.0", "https://github.com/safedep/vet/releases/tag/v2.0.0", http.StatusOK)
	defer server.Close()

	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ch := checker.CheckAsync(context.Background(), "v1.0.0")
	result := <-ch
	require.NotNil(t, result)
	assert.True(t, result.UpdateAvailable)
	assert.Equal(t, "v2.0.0", result.LatestVersion)
}

func TestCheckAsync_ErrorReturnsNil(t *testing.T) {
	server := newTestServer(t, "", "", http.StatusInternalServerError)
	defer server.Close()

	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		BaseURL: server.URL,
	})
	require.NoError(t, err)

	ch := checker.CheckAsync(context.Background(), "v1.0.0")
	result := <-ch
	assert.Nil(t, result)
}

func TestNewChecker_Defaults(t *testing.T) {
	checker, err := NewChecker(Config{Owner: "safedep", Repo: "vet"})
	require.NoError(t, err)
	assert.Equal(t, defaultBaseURL, checker.config.BaseURL)
	assert.Equal(t, defaultTimeout, checker.config.Timeout)
}

func TestNewChecker_CustomConfig(t *testing.T) {
	checker, err := NewChecker(Config{
		Owner:   "safedep",
		Repo:    "vet",
		Timeout: 10 * time.Second,
		BaseURL: "https://custom.api.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://custom.api.com", checker.config.BaseURL)
	assert.Equal(t, 10*time.Second, checker.config.Timeout)
}

func TestNewChecker_MissingOwner(t *testing.T) {
	_, err := NewChecker(Config{Repo: "vet"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "owner is required")
}

func TestNewChecker_MissingRepo(t *testing.T) {
	_, err := NewChecker(Config{Owner: "safedep"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repo is required")
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{"newer available", "v1.0.0", "v2.0.0", true},
		{"same version", "v1.0.0", "v1.0.0", false},
		{"ahead of latest", "v2.0.0", "v1.0.0", false},
		{"without v prefix", "1.0.0", "2.0.0", true},
		{"patch update", "v1.0.0", "v1.0.1", true},
		{"invalid current", "invalid", "v1.0.0", false},
		{"invalid latest", "v1.0.0", "invalid", false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isNewer(tt.current, tt.latest))
		})
	}
}
