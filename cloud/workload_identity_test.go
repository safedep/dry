package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubActionsSource(t *testing.T) {
	// elsewhere stands for another host. A redirect must not reach it.
	var elsewhereHits atomic.Int32
	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		elsewhereHits.Add(1)
		_, err := w.Write([]byte(`{"value":"stolen.token.value"}`))
		assert.NoError(t, err)
	}))
	t.Cleanup(elsewhere.Close)

	cases := []struct {
		name      string
		status    int
		header    map[string]string
		body      string
		wantToken string
		wantErr   string
	}{
		{
			name:      "returns the ID token",
			status:    http.StatusOK,
			body:      `{"count":1,"value":"header.payload.signature"}`,
			wantToken: "header.payload.signature",
		},
		{
			name:    "fails on an error status",
			status:  http.StatusInternalServerError,
			wantErr: "returned status 500",
		},
		{
			name:    "fails on a response with no token",
			status:  http.StatusOK,
			body:    `{"count":0}`,
			wantErr: "holds no token",
		},
		{
			name:    "does not follow a redirect",
			status:  http.StatusFound,
			header:  map[string]string{"Location": elsewhere.URL + "/token"},
			wantErr: "returned status 302",
		},
		{
			name:    "fails on an oversized response",
			status:  http.StatusOK,
			body:    `{"value":"` + strings.Repeat("a", maxIDTokenResponseBytes) + `"}`,
			wantErr: "read the GitHub Actions ID token response",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuth, gotAudience, gotAPIVersion string
			endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				gotAudience = r.URL.Query().Get("audience")
				gotAPIVersion = r.URL.Query().Get("api-version")
				for k, v := range tc.header {
					w.Header().Set(k, v)
				}
				w.WriteHeader(tc.status)
				_, err := w.Write([]byte(tc.body))
				assert.NoError(t, err)
			}))
			t.Cleanup(endpoint.Close)

			t.Setenv(insecureTransportEnvVar, "")
			t.Setenv(githubActionsIDTokenRequestURLEnvVar, endpoint.URL+"/token?api-version=2.0")
			t.Setenv(githubActionsIDTokenRequestTokenEnvVar, "request-token")

			source, err := DetectWorkloadIdentitySource()
			require.NoError(t, err)
			// Trust the test certificate. The client keeps its redirect rule.
			gh, ok := source.(*githubActionsSource)
			require.True(t, ok)
			gh.httpClient.Transport = endpoint.Client().Transport

			token, err := source.Token(context.Background(), WorkloadIdentityAudience)

			assert.Equal(t, "Bearer request-token", gotAuth)
			assert.Equal(t, "sts.safedep.io", gotAudience)
			assert.Equal(t, "2.0", gotAPIVersion)
			assert.Zero(t, elsewhereHits.Load())
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				assert.NotContains(t, err.Error(), "request-token")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantToken, token.Value)
			assert.Equal(t, controltowerv1.SubjectTokenType_SUBJECT_TOKEN_TYPE_GITHUB_ACTIONS_OIDC, token.Type)
		})
	}
}

func TestGitHubActionsSource_RequiresHTTPS(t *testing.T) {
	t.Setenv(githubActionsIDTokenRequestURLEnvVar, "http://pipelines.example.test/token?api-version=2.0")
	t.Setenv(githubActionsIDTokenRequestTokenEnvVar, "request-token")

	t.Setenv(insecureTransportEnvVar, "")
	_, err := DetectWorkloadIdentitySource()
	require.ErrorContains(t, err, "ACTIONS_ID_TOKEN_REQUEST_URL must use https")
	assert.NotErrorIs(t, err, ErrNoWorkloadIdentity, "a bad URL is not a missing identity")

	t.Setenv(insecureTransportEnvVar, "true")
	_, err = DetectWorkloadIdentitySource()
	assert.NoError(t, err, "the switch for local development allows http")
}

func TestDetectWorkloadIdentitySource_None(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		wantMsg string
	}{
		{name: "no variables", wantMsg: ErrNoWorkloadIdentity.Error()},
		{
			name:    "request URL only",
			env:     map[string]string{githubActionsIDTokenRequestURLEnvVar: "https://example.test/token"},
			wantMsg: "ACTIONS_ID_TOKEN_REQUEST_URL is set but ACTIONS_ID_TOKEN_REQUEST_TOKEN is not",
		},
		{
			name:    "request token only",
			env:     map[string]string{githubActionsIDTokenRequestTokenEnvVar: "request-token"},
			wantMsg: "ACTIONS_ID_TOKEN_REQUEST_TOKEN is set but ACTIONS_ID_TOKEN_REQUEST_URL is not",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{githubActionsIDTokenRequestURLEnvVar, githubActionsIDTokenRequestTokenEnvVar} {
				t.Setenv(k, tc.env[k])
			}

			_, err := DetectWorkloadIdentitySource()
			require.ErrorIs(t, err, ErrNoWorkloadIdentity)
			assert.ErrorContains(t, err, tc.wantMsg)
			assert.NotContains(t, err.Error(), "request-token")
		})
	}
}
