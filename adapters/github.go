package adapters

import (
	"net/http"
	"os"

	"github.com/google/go-github/v70/github"
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
