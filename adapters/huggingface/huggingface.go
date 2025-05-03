package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	defaultHuggingFaceHubAPIBaseURL = "https://huggingface.co/api"
	defaultTimeout                  = 30 * time.Second
)

// HuggingFaceHubClient defines the interface for interacting with the HuggingFace Hub API
type HuggingFaceHubClient interface {
	// GetModel fetches metadata for a specific model from HuggingFace Hub
	GetModel(ctx context.Context, owner, name string) (*HuggingFaceModel, error)

	// GetDataset fetches metadata for a specific dataset from HuggingFace Hub
	GetDataset(ctx context.Context, owner, name string) (*HuggingFaceDataset, error)
}

type huggingFaceHubClientImpl struct {
	baseURL    string
	httpClient *http.Client
	apiToken   string
}

// HuggingFaceHubClientOption defines a function to configure the client
type HuggingFaceHubClientOption func(*huggingFaceHubClientImpl)

// WithBaseURL sets a custom base URL for the HuggingFace Hub API
func WithBaseURL(baseURL string) HuggingFaceHubClientOption {
	return func(c *huggingFaceHubClientImpl) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets a custom timeout for HTTP requests
func WithTimeout(timeout time.Duration) HuggingFaceHubClientOption {
	return func(c *huggingFaceHubClientImpl) {
		c.httpClient.Timeout = timeout
	}
}

// WithAPIToken sets the authentication token for the HuggingFace Hub API
func WithAPIToken(token string) HuggingFaceHubClientOption {
	return func(c *huggingFaceHubClientImpl) {
		c.apiToken = token
	}
}

// WithHTTPClient sets a custom HTTP client for the HuggingFace Hub API
func WithHTTPClient(httpClient *http.Client) HuggingFaceHubClientOption {
	return func(c *huggingFaceHubClientImpl) {
		c.httpClient = httpClient
	}
}

var _ HuggingFaceHubClient = &huggingFaceHubClientImpl{}

// NewHuggingFaceHubClient creates a new HuggingFaceHubClient
func NewHuggingFaceHubClient(opts ...HuggingFaceHubClientOption) *huggingFaceHubClientImpl {
	client := &huggingFaceHubClientImpl{
		baseURL: defaultHuggingFaceHubAPIBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	// Auto-configure the API token from the environment variable
	// if not explicitly set by the user
	if client.apiToken == "" {
		client.apiToken = os.Getenv("HF_TOKEN")
	}

	return client
}

// GetModel fetches metadata for a specific model from HuggingFace Hub
func (c *huggingFaceHubClientImpl) GetModel(ctx context.Context, owner, name string) (*HuggingFaceModel, error) {
	path := fmt.Sprintf("/models/%s/%s", url.PathEscape(owner), url.PathEscape(name))

	data, err := c.doRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var model HuggingFaceModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, wrap(err, ErrInvalidResponse, "failed to parse model response")
	}

	model.RawResponse = data
	return &model, nil
}

// GetDataset fetches metadata for a specific dataset from HuggingFace Hub
func (c *huggingFaceHubClientImpl) GetDataset(ctx context.Context, owner, name string) (*HuggingFaceDataset, error) {
	path := fmt.Sprintf("/datasets/%s/%s", url.PathEscape(owner), url.PathEscape(name))

	data, err := c.doRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var dataset HuggingFaceDataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return nil, wrap(err, ErrInvalidResponse, "failed to parse dataset response")
	}

	dataset.RawResponse = data
	return &dataset, nil
}

// doRequest performs an HTTP request to the HuggingFace Hub API
func (c *huggingFaceHubClientImpl) doRequest(ctx context.Context, path string) ([]byte, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, wrap(err, ErrInvalidRequest, "failed to create request")
	}

	req.Header.Set("Accept", "application/json")

	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrap(err, ErrNetworkError, "failed to send request")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrap(err, ErrIOError, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: HuggingFace Hub API error - HTTP %d: %s",
			ErrAPIError, resp.StatusCode, string(data))
	}

	return data, nil
}
