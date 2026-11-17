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
// Do not write Value to a log.
type WorkloadToken struct {
	Value string
	Type  controltowerv1.SubjectTokenType
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
// environment. It returns ErrNoWorkloadIdentity when the environment has
// none. It makes no network call.
func DetectWorkloadIdentitySource() (WorkloadIdentitySource, error) {
	detectors := []func() (WorkloadIdentitySource, error){
		NewGitHubActionsSource,
	}

	for _, detect := range detectors {
		source, err := detect()
		if errors.Is(err, ErrNoWorkloadIdentity) {
			continue
		}
		return source, err
	}

	return nil, ErrNoWorkloadIdentity
}

type githubActionsSource struct {
	requestURL   string
	requestToken string
	httpClient   *http.Client
}

// NewGitHubActionsSource returns the source of GitHub Actions OIDC ID tokens.
// It reads ACTIONS_ID_TOKEN_REQUEST_URL and ACTIONS_ID_TOKEN_REQUEST_TOKEN,
// and returns ErrNoWorkloadIdentity when either is not set.
func NewGitHubActionsSource() (WorkloadIdentitySource, error) {
	requestURL := os.Getenv(githubActionsIDTokenRequestURLEnvVar)
	requestToken := os.Getenv(githubActionsIDTokenRequestTokenEnvVar)
	if requestURL == "" || requestToken == "" {
		return nil, fmt.Errorf("%w: %s and %s are not both set", ErrNoWorkloadIdentity,
			githubActionsIDTokenRequestURLEnvVar, githubActionsIDTokenRequestTokenEnvVar)
	}

	return &githubActionsSource{
		requestURL:   requestURL,
		requestToken: requestToken,
		httpClient:   &http.Client{Timeout: githubActionsIDTokenTimeout},
	}, nil
}

func (s *githubActionsSource) Token(ctx context.Context, audience string) (WorkloadToken, error) {
	u, err := url.Parse(s.requestURL)
	if err != nil {
		return WorkloadToken{}, fmt.Errorf("cloud: parse %s: %w", githubActionsIDTokenRequestURLEnvVar, err)
	}

	// The request URL already carries a query, such as api-version.
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
