package adapters

import (
	"fmt"
	"net/http"
	"os"
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

	// AppAuthenticationPrivateKey is the PEM encoded private key for a GitHub App
	// JWT based authentication
	// https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
	AppAuthenticationPrivateKey []byte

	// Enterprise GitHub URLs
	// eg. EnterpriseBaseURL = "https://github.yourdomain.com/api/v3"
	// eg. EnterpriseUploadURL = "https://github.yourdomain.com/api/uploads"
	EnterpriseBaseURL   string
	EnterpriseUploadURL string

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

	// Try to read the private key from the environment variable
	// Fallback to reading from a file if the env var is not set
	appAuthenticationPrivateKey := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if appAuthenticationPrivateKey == "" {
		fromFile := os.Getenv("GITHUB_APP_PRIVATE_KEY_FILE")
		if fromFile != "" {
			data, err := os.ReadFile(fromFile)
			if err != nil {
				log.Warnf("Failed to read GITHUB_APP_JWT_FILE: %v", err)
			} else {
				appAuthenticationPrivateKey = string(data)
			}
		}
	}

	return GitHubClientConfig{
		Token:                       token,
		ClientId:                    clientId,
		ClientSecret:                clientSecret,
		EnterpriseBaseURL:           enterpriseBaseURL,
		EnterpriseUploadURL:         enterpriseUploadURL,
		HTTPClient:                  http.DefaultClient,
		AppAuthenticationPrivateKey: []byte(appAuthenticationPrivateKey),
	}
}

type basicAuthTransportWrapper struct {
	Transport http.RoundTripper
	Username  string
	Password  string
}

type GithubClient struct {
	Client *github.Client
	Config GitHubClientConfig
}

func (b *basicAuthTransportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(b.Username, b.Password)
	return b.Transport.RoundTrip(req)
}

func NewGithubClient(config GitHubClientConfig) (*GithubClient, error) {
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}

	client := github.NewClient(config.HTTPClient)

	// Configure enterprise URLs if provided
	if config.EnterpriseBaseURL != "" && config.EnterpriseUploadURL != "" {
		log.Debugf("Using GitHub Enterprise URLs: base=%s, upload=%s", config.EnterpriseBaseURL, config.EnterpriseUploadURL)
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

// CreateAppAuthenticationJWT creates a JWT for GitHub App authentication
// following instructions from: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-json-web-token-jwt-for-a-github-app
func (g *GithubClient) CreateAppAuthenticationJWT(appID string) (string, error) {
	if len(g.Config.AppAuthenticationPrivateKey) == 0 {
		return "", fmt.Errorf("app authentication private key is not configured")
	}

	// Parse the RSA private key from PEM
	key, err := jwt.ParseRSAPrivateKeyFromPEM(g.Config.AppAuthenticationPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to parse app authentication private key: %v", err)
	}

	// Create the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)), // Issued at time, 1 minute in the past to allow for clock skew
		"exp": jwt.NewNumericDate(time.Now().Add(10 * time.Minute)), // Expiration time, 10 minutes from now
		"iss": appID,                                                // GitHub App ID
	})

	// Sign the token with the private key
	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}
