package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
)

// WorkloadIdentityAudience is the audience that SafeDep Cloud requires in a
// workload identity token.
const WorkloadIdentityAudience = "sts.safedep.io"

const (
	// GitHub Actions sets both in a job that has the permission
	// "id-token: write".
	githubActionsIDTokenRequestURLEnvVar   = "ACTIONS_ID_TOKEN_REQUEST_URL"
	githubActionsIDTokenRequestTokenEnvVar = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"

	githubActionsIDTokenTimeout = 30 * time.Second

	// GitHub returns a JSON object with one JWT of a few KiB. The limit stops
	// a broken endpoint from filling memory.
	maxIDTokenResponseBytes = 64 << 10
)

// ErrNoWorkloadIdentity is returned when the environment has no workload
// identity, for example a GitHub Actions job without "id-token: write".
var ErrNoWorkloadIdentity = errors.New("cloud: no workload identity in the environment")

// WorkloadToken is a token that a platform signs for the running workload.
// Do not write Value to a log. String and GoString leave it out.
type WorkloadToken struct {
	Value string
	Type  controltowerv1.SubjectTokenType
}

// String describes the token without its value.
func (t WorkloadToken) String() string {
	return fmt.Sprintf("workload token of type %s", t.Type)
}

// GoString keeps the token value out of %#v.
func (t WorkloadToken) GoString() string {
	return t.String()
}

// WorkloadIdentitySource gets a token that a platform signs for the running
// workload, such as a GitHub Actions OIDC ID token.
//
// Token returns a new token for each call. The exchange accepts a token one
// time only, so a caller must not keep a token for a later exchange.
type WorkloadIdentitySource interface {
	Token(ctx context.Context, audience string) (WorkloadToken, error)
}

// DetectWorkloadIdentitySource returns the workload identity source of the
// environment. It returns an error that wraps ErrNoWorkloadIdentity when the
// environment has none. When a platform sets only part of its variables, the
// error names the missing one. It makes no network call.
func DetectWorkloadIdentitySource() (WorkloadIdentitySource, error) {
	detectors := []func() (WorkloadIdentitySource, error){
		NewGitHubActionsSource,
	}

	none := ErrNoWorkloadIdentity
	for _, detect := range detectors {
		source, err := detect()
		if !errors.Is(err, ErrNoWorkloadIdentity) {
			return source, err
		}
		// A wrapped error names the missing variable. Keep it for the caller.
		if err != ErrNoWorkloadIdentity {
			none = err
		}
	}

	return nil, none
}

type githubActionsSource struct {
	requestURL   *url.URL
	requestToken string
	httpClient   *http.Client
}

// NewGitHubActionsSource returns the source of GitHub Actions OIDC ID tokens.
// It reads ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN.
// It returns ErrNoWorkloadIdentity when neither is set, and an error that
// wraps ErrNoWorkloadIdentity and names the missing one when one is set.
//
// The request URL must use https, because the request carries the request
// token. INSECURE_GRPC_CLIENT_USE_INSECURE_TRANSPORT, the switch for local
// development, also allows http.
func NewGitHubActionsSource() (WorkloadIdentitySource, error) {
	requestURL := os.Getenv(githubActionsIDTokenRequestURLEnvVar)
	requestToken := os.Getenv(githubActionsIDTokenRequestTokenEnvVar)

	switch {
	case requestURL == "" && requestToken == "":
		return nil, ErrNoWorkloadIdentity
	case requestURL == "":
		return nil, fmt.Errorf("%w: %s is set but %s is not", ErrNoWorkloadIdentity,
			githubActionsIDTokenRequestTokenEnvVar, githubActionsIDTokenRequestURLEnvVar)
	case requestToken == "":
		return nil, fmt.Errorf("%w: %s is set but %s is not", ErrNoWorkloadIdentity,
			githubActionsIDTokenRequestURLEnvVar, githubActionsIDTokenRequestTokenEnvVar)
	}

	u, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("cloud: parse %s: %w", githubActionsIDTokenRequestURLEnvVar, err)
	}
	allowed := u.Scheme == "https" || (u.Scheme == "http" && insecureTransport())
	if !allowed {
		return nil, fmt.Errorf("cloud: %s must use https, not %q", githubActionsIDTokenRequestURLEnvVar, u.Scheme)
	}

	return &githubActionsSource{
		requestURL:   u,
		requestToken: requestToken,
		httpClient: &http.Client{
			Timeout: githubActionsIDTokenTimeout,
			// A redirect would send the request token to another host.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// String describes the source without its request token.
func (s *githubActionsSource) String() string {
	return "GitHub Actions workload identity source"
}

// GoString keeps the request token out of %#v.
func (s *githubActionsSource) GoString() string {
	return s.String()
}

func (s *githubActionsSource) Token(ctx context.Context, audience string) (WorkloadToken, error) {
	// The request URL already carries a query, such as api-version.
	u := *s.requestURL
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return WorkloadToken{}, fmt.Errorf("cloud: build the GitHub Actions ID token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.requestToken)
	req.Header.Set("Accept", "application/json")

	res, err := s.httpClient.Do(req)
	if err != nil {
		return WorkloadToken{}, fmt.Errorf("cloud: request the GitHub Actions ID token: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return WorkloadToken{}, fmt.Errorf("cloud: the GitHub Actions ID token request returned status %d", res.StatusCode)
	}

	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxIDTokenResponseBytes)).Decode(&body); err != nil {
		return WorkloadToken{}, fmt.Errorf("cloud: read the GitHub Actions ID token response: %w", err)
	}
	if body.Value == "" {
		return WorkloadToken{}, errors.New("cloud: the GitHub Actions ID token response holds no token")
	}

	return WorkloadToken{
		Value: body.Value,
		Type:  controltowerv1.SubjectTokenType_SUBJECT_TOKEN_TYPE_GITHUB_ACTIONS_OIDC,
	}, nil
}
