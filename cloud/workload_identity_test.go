package cloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubActionsSource(t *testing.T) {
	cases := []struct {
		name      string
		status    int
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuth, gotAudience, gotAPIVersion string
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				gotAudience = r.URL.Query().Get("audience")
				gotAPIVersion = r.URL.Query().Get("api-version")
				w.WriteHeader(tc.status)
				_, err := w.Write([]byte(tc.body))
				assert.NoError(t, err)
			}))
			t.Cleanup(endpoint.Close)

			t.Setenv(githubActionsIDTokenRequestURLEnvVar, endpoint.URL+"/token?api-version=2.0")
			t.Setenv(githubActionsIDTokenRequestTokenEnvVar, "request-token")

			source, err := DetectWorkloadIdentitySource()
			require.NoError(t, err)

			token, err := source.Token(context.Background(), WorkloadIdentityAudience)

			assert.Equal(t, "Bearer request-token", gotAuth)
			assert.Equal(t, "sts.safedep.io", gotAudience)
			assert.Equal(t, "2.0", gotAPIVersion)
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

func TestDetectWorkloadIdentitySource_None(t *testing.T) {
	cases := map[string]map[string]string{
		"no variables":       {},
		"request URL only":   {githubActionsIDTokenRequestURLEnvVar: "https://example.test/token"},
		"request token only": {githubActionsIDTokenRequestTokenEnvVar: "request-token"},
	}

	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			for _, k := range []string{githubActionsIDTokenRequestURLEnvVar, githubActionsIDTokenRequestTokenEnvVar} {
				t.Setenv(k, env[k])
			}

			_, err := DetectWorkloadIdentitySource()
			assert.ErrorIs(t, err, ErrNoWorkloadIdentity)
		})
	}
}
