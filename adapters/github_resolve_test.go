package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBranchSHA = "0123456789abcdef0123456789abcdef01234567"
	testTagSHA    = "fedcba9876543210fedcba9876543210fedcba98"
)

// newTestGithubClient serves a minimal GitHub REST API from an httptest server
// and returns a client pointed at it through the enterprise base URL.
func newTestGithubClient(t *testing.T) (*GithubClient, *atomic.Int32) {
	t.Helper()

	var requests atomic.Int32
	mux := http.NewServeMux()
	handle := func(pattern string, body any) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
			require.NoError(t, json.NewEncoder(w).Encode(body))
		})
	}

	handle("GET /api/v3/repos/safedep/dry", map[string]string{"default_branch": "trunk"})

	// Keyed by the decoded ref, so the test proves reserved characters
	// survive the round trip through the request path.
	commits := map[string]string{
		"trunk":       testBranchSHA,
		"main":        testBranchSHA,
		"v1.0.0":      testTagSHA,
		"release/1.0": testTagSHA,
		"release#1":   testTagSHA,
		"hot fix?":    testTagSHA,
	}
	mux.HandleFunc("GET /api/v3/repos/safedep/dry/commits/", func(w http.ResponseWriter, r *http.Request) {
		ref := strings.TrimPrefix(r.URL.Path, "/api/v3/repos/safedep/dry/commits/")
		sha, ok := commits[ref]
		if !ok {
			http.NotFound(w, r)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"sha": sha}))
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	client, err := NewGithubClient(GitHubClientConfig{
		EnterpriseBaseURL:   server.URL + "/api/v3/",
		EnterpriseUploadURL: server.URL + "/api/uploads/",
	})
	require.NoError(t, err)

	return client, &requests
}

func TestGithubClientResolveCommitSHA(t *testing.T) {
	cases := []struct {
		name         string
		ref          string
		wantSHA      string
		wantRequests int32
		wantErr      string
	}{
		{name: "branch", ref: "main", wantSHA: testBranchSHA, wantRequests: 1},
		{name: "tag", ref: "v1.0.0", wantSHA: testTagSHA, wantRequests: 1},
		{name: "branch with slash", ref: "release/1.0", wantSHA: testTagSHA, wantRequests: 1},
		{name: "tag with hash", ref: "release#1", wantSHA: testTagSHA, wantRequests: 1},
		{name: "branch with space and question mark", ref: "hot fix?", wantSHA: testTagSHA, wantRequests: 1},
		{name: "empty ref uses default branch", ref: "", wantSHA: testBranchSHA, wantRequests: 2},
		{name: "sha1 passes through", ref: testTagSHA, wantSHA: testTagSHA},
		{name: "upper case sha1 passes through", ref: "FEDCBA9876543210FEDCBA9876543210FEDCBA98", wantSHA: "FEDCBA9876543210FEDCBA9876543210FEDCBA98"},
		{name: "sha256 passes through", ref: testBranchSHA + "0123456789abcdef01234567", wantSHA: testBranchSHA + "0123456789abcdef01234567"},
		{name: "unknown ref", ref: "missing", wantRequests: 1, wantErr: `failed to resolve ref "missing" of safedep/dry`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, requests := newTestGithubClient(t)

			sha, err := client.ResolveCommitSHA(context.Background(), "safedep", "dry", tc.ref)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantSHA, sha)
			}

			assert.Equal(t, tc.wantRequests, requests.Load())
		})
	}
}

func TestGithubClientResolveCommitSHA_repositoryLookupFails(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)

	client, err := NewGithubClient(GitHubClientConfig{
		EnterpriseBaseURL:   server.URL + "/api/v3/",
		EnterpriseUploadURL: server.URL + "/api/uploads/",
	})
	require.NoError(t, err)

	_, err = client.ResolveCommitSHA(context.Background(), "safedep", "dry", "")
	require.ErrorContains(t, err, "failed to get repository safedep/dry")
}

func TestIsCommitSHA(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{in: testBranchSHA, want: true},
		{in: testBranchSHA + "0123456789abcdef01234567", want: true},
		{in: "ABCDEF0123456789ABCDEF0123456789ABCDEF01", want: true},
		{in: "main", want: false},
		{in: "", want: false},
		{in: testBranchSHA[:39], want: false},
		{in: testBranchSHA[:39] + "g", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, IsCommitSHA(tc.in))
		})
	}
}
