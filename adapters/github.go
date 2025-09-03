package adapters

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v74/github"
	"github.com/safedep/dry/log"
)

type GitHubClientConfig struct {
	// PAT / Token based authentication
	Token string

	// ClientId and ClientSecret for basic authentication
	// https://docs.github.com/en/rest/authentication/authenticating-to-the-rest-api#using-basic-authentication
	// App credentials usually have higher rate limits
	ClientId     string
	ClientSecret string

	// Enterprise GitHub URLs
	// eg. EnterpriseBaseURL = "https://github.yourdomain.com/api/v3"
	// eg. EnterpriseUploadURL = "https://github.yourdomain.com/api/uploads"
	EnterpriseBaseURL   string
	EnterpriseUploadURL string

	// This is useful when we want to supply a client that
	// can handle rate limiting, etc.
	HTTPClient *http.Client
}

// GitHubAppClientConfig contains configuration specific to GitHub App authentication
type GitHubAppClientConfig struct {
	// AppAuthenticationPrivateKey is the PEM encoded private key for a GitHub App
	// JWT based authentication
	// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
	AppAuthenticationPrivateKey []byte
	AppID                       string

	// Cache JWT token until it expires
	EnableJWTTokenCache bool

	// Credentials for GitHub App
	ClientId     string
	ClientSecret string

	// This is useful when we want to supply a client that
	// can handle rate limiting, etc.
	HTTPClient *http.Client
}

func DefaultGitHubClientConfig() GitHubClientConfig {
	token := os.Getenv("GITHUB_TOKEN")

	clientId, clientSecret := os.Getenv("GITHUB_CLIENT_ID"),
		os.Getenv("GITHUB_CLIENT_SECRET")

	enterpriseBaseURL, enterpriseUploadURL := os.Getenv("GITHUB_BASE_URL"),
		os.Getenv("GITHUB_UPLOAD_URL")

	return GitHubClientConfig{
		Token:               token,
		ClientId:            clientId,
		ClientSecret:        clientSecret,
		EnterpriseBaseURL:   enterpriseBaseURL,
		EnterpriseUploadURL: enterpriseUploadURL,
		HTTPClient:          http.DefaultClient,
	}
}

// DefaultGitHubAppClientConfig creates a default configuration for GitHub App authentication
// using environment variables
func DefaultGitHubAppClientConfig() GitHubAppClientConfig {
	// Try to read the private key from the environment variable
	// Fallback to reading from a file if the env var is not set
	appAuthenticationPrivateKey := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if appAuthenticationPrivateKey == "" {
		fromFile := os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE")
		if fromFile != "" {
			data, err := os.ReadFile(fromFile)
			if err != nil {
				log.Warnf("Failed to read GITHUB_APP_PRIVATE_KEY_FILE: %v", err)
			} else {
				appAuthenticationPrivateKey = string(data)
			}
		}
	}

	appID := os.Getenv("GITHUB_APP_ID")
	clientId := os.Getenv("GITHUB_APP_CLIENT_ID")
	clientSecret := os.Getenv("GITHUB_APP_CLIENT_SECRET")

	return GitHubAppClientConfig{
		AppAuthenticationPrivateKey: []byte(appAuthenticationPrivateKey),
		AppID:                       appID,
		EnableJWTTokenCache:         true,
		ClientId:                    clientId,
		ClientSecret:                clientSecret,
		HTTPClient:                  http.DefaultClient,
	}
}

type GithubClient struct {
	Client *github.Client
	Config GitHubClientConfig
}

// GitHubAppClient is a specialized client for GitHub App authentication
type GitHubAppClient struct {
	Config GitHubAppClientConfig

	m                sync.Mutex
	parsedPrivateKey *rsa.PrivateKey
	cachedToken      string
	cachedTokenExp   time.Time
}

type basicAuthTransportWrapper struct {
	Transport http.RoundTripper
	Username  string
	Password  string
}

func (b *basicAuthTransportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(b.Username, b.Password)
	return b.Transport.RoundTrip(req)
}

type jwtAuthTransportWrapper struct {
	Transport http.RoundTripper
	Token     string
}

func (j *jwtAuthTransportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+j.Token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	return j.Transport.RoundTrip(req)
}

func NewGithubClient(config GitHubClientConfig) (*GithubClient, error) {
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}

	client := github.NewClient(config.HTTPClient)

	// Configure enterprise URLs if provided
	if config.EnterpriseBaseURL != "" && config.EnterpriseUploadURL != "" {
		log.Debugf("Using GitHub Enterprise URLs: base=%s, upload=%s",
			config.EnterpriseBaseURL, config.EnterpriseUploadURL)

		var err error
		client, err = client.WithEnterpriseURLs(config.EnterpriseBaseURL, config.EnterpriseUploadURL)
		if err != nil {
			return nil, err
		}
	}

	// Client credentials have highest precedence
	// for client authentication
	if config.ClientId != "" && config.ClientSecret != "" {
		log.Debugf("Using client credentials for GitHub authentication")
		client.Client().Transport = &basicAuthTransportWrapper{
			Transport: client.Client().Transport,
			Username:  config.ClientId,
			Password:  config.ClientSecret,
		}
	} else if config.Token != "" {
		log.Debugf("Using token for GitHub authentication")
		client = client.WithAuthToken(config.Token)
	} else {
		log.Debugf("Using unauthenticated Github client")
	}

	return &GithubClient{
		Client: client,
		Config: config,
	}, nil
}

// NewGitHubAppClient creates a new GitHub App client with JWT-based authentication
func NewGitHubAppClient(config GitHubAppClientConfig) (*GitHubAppClient, error) {
	if len(config.AppAuthenticationPrivateKey) == 0 {
		return nil, fmt.Errorf("app authentication private key is not configured")
	}

	if config.AppID == "" {
		return nil, fmt.Errorf("app ID is not configured")
	}

	key, err := jwt.ParseRSAPrivateKeyFromPEM(config.AppAuthenticationPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse app authentication private key: %v", err)
	}

	return &GitHubAppClient{
		Config:           config,
		m:                sync.Mutex{},
		parsedPrivateKey: key,
		cachedToken:      "",
	}, nil
}

// CreateAppAuthenticationJWT creates a JWT for GitHub App authentication
// following instructions from: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
func (g *GitHubAppClient) CreateAppAuthenticationJWT() (string, error) {
	if g.Config.EnableJWTTokenCache {
		g.m.Lock()
		defer g.m.Unlock()

		// Return cached token if it's still valid while adjusting for clock skew
		if g.cachedToken != "" && time.Now().Before(g.cachedTokenExp) {
			return g.cachedToken, nil
		}
	}

	// Convert AppID from string to int for JWT issuer claim
	// GitHub requires the issuer to be a numeric App ID, not a string
	appID, err := strconv.Atoi(g.Config.AppID)
	if err != nil {
		return "", fmt.Errorf("invalid app ID format, must be numeric: %v", err)
	}

	// Create the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)), // Issued at time, 1 minute in the past to allow for clock skew
		"exp": jwt.NewNumericDate(time.Now().Add(10 * time.Minute)), // Expiration time, 10 minutes from now
		"iss": appID,                                                // GitHub App ID as integer
	})

	// Sign the token with the private key
	signedToken, err := token.SignedString(g.parsedPrivateKey)
	if err != nil {
		return "", err
	}

	if g.Config.EnableJWTTokenCache {
		// The mutex is already locked above
		g.cachedToken = signedToken
		g.cachedTokenExp = time.Now().Add(9 * time.Minute) // Cache for 9 minutes to allow for clock skew
	}

	return signedToken, nil
}

// AuthenticatedClient returns a GitHub client authenticated using the Github App
// clientId and clientSecret. We use different functions because we need
// different round trippers for JWT vs basic auth.
func (g *GitHubAppClient) AuthenticatedClient() (*github.Client, error) {
	if g.Config.ClientId == "" || g.Config.ClientSecret == "" {
		return nil, fmt.Errorf("client ID and client secret must be provided for authenticated client")
	}

	httpClient := g.Config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Create a new client to avoid modifying the original client's transport
	authClient := github.NewClient(httpClient)

	// Use basic authentication with clientId and clientSecret
	authClient.Client().Transport = &basicAuthTransportWrapper{
		Transport: authClient.Client().Transport,
		Username:  g.Config.ClientId,
		Password:  g.Config.ClientSecret,
	}

	return authClient, nil
}

// AuthenticatedAppClient returns a GitHub client authenticated as the GitHub App
// using a JWT token.
func (g *GitHubAppClient) AuthenticatedAppClient() (*github.Client, error) {
	jwtToken, err := g.CreateAppAuthenticationJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to create app authentication JWT: %v", err)
	}

	httpClient := g.Config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Create a new client to avoid modifying the original client's transport
	appClient := github.NewClient(httpClient)

	// Use JWT authentication for GitHub App
	appClient.Client().Transport = &jwtAuthTransportWrapper{
		Transport: appClient.Client().Transport,
		Token:     jwtToken,
	}

	return appClient, nil
}
