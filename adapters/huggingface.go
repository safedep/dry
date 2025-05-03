package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// HuggingFaceModel represents metadata about a model in HuggingFace Hub
type HuggingFaceModel struct {
	ID               string            `json:"id"`               // Unique identifier of the model (owner/name)
	ModelName        string            `json:"modelId"`          // Name of the model
	Author           string            `json:"author"`           // Author of the model
	Tags             []string          `json:"tags"`             // Tags associated with the model
	Downloads        int64             `json:"downloads"`        // Number of downloads
	Likes            int               `json:"likes"`            // Number of likes
	CreatedAt        string            `json:"createdAt"`        // Creation date
	LastModified     string            `json:"lastModified"`     // Last modification date
	Private          bool              `json:"private"`          // Whether the model is private
	PipelineTag      string            `json:"pipeline_tag"`     // Pipeline tag
	Library          string            `json:"library"`          // Associated library
	CardData         map[string]any    `json:"cardData"`         // Model card data
	SiblingModels    []string          `json:"siblings"`         // Sibling models
	Config           map[string]any    `json:"config"`           // Model configuration
	SafeTensors      bool              `json:"safetensors"`      // Whether using safetensors
	License          string            `json:"license"`          // License information
	Metrics          []MetricInfo      `json:"metrics"`          // Model metrics
	RawResponse      json.RawMessage   `json:"-"`                // Raw response from the API
}

// HuggingFaceDataset represents metadata about a dataset in HuggingFace Hub
type HuggingFaceDataset struct {
	ID              string            `json:"id"`               // Unique identifier of the dataset (owner/name)
	DatasetName     string            `json:"datasetId"`        // Name of the dataset
	Author          string            `json:"author"`           // Author of the dataset
	Tags            []string          `json:"tags"`             // Tags associated with the dataset
	Downloads       int64             `json:"downloads"`        // Number of downloads
	Likes           int               `json:"likes"`            // Number of likes
	CreatedAt       string            `json:"createdAt"`        // Creation date
	LastModified    string            `json:"lastModified"`     // Last modification date
	Private         bool              `json:"private"`          // Whether the dataset is private
	CardData        map[string]any    `json:"cardData"`         // Dataset card data
	SiblingDatasets []string          `json:"siblings"`         // Sibling datasets
	Description     string            `json:"description"`      // Description of the dataset
	Citation        string            `json:"citation"`         // Citation information
	License         string            `json:"license"`          // License information
	Size            int64             `json:"size"`             // Size of the dataset
	RawResponse     json.RawMessage   `json:"-"`                // Raw response from the API
}

// MetricInfo represents metrics information for a model
type MetricInfo struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}

// HuggingFaceHubClientImpl implements the HuggingFaceHubClient interface
type HuggingFaceHubClientImpl struct {
	baseURL    string
	httpClient *http.Client
	apiToken   string
}

// HuggingFaceHubClientOption defines a function to configure the HuggingFaceHubClientImpl
type HuggingFaceHubClientOption func(*HuggingFaceHubClientImpl)

// WithBaseURL sets a custom base URL for the HuggingFace Hub API
func WithBaseURL(baseURL string) HuggingFaceHubClientOption {
	return func(c *HuggingFaceHubClientImpl) {
		c.baseURL = baseURL
	}
}

// WithTimeout sets a custom timeout for HTTP requests
func WithTimeout(timeout time.Duration) HuggingFaceHubClientOption {
	return func(c *HuggingFaceHubClientImpl) {
		c.httpClient.Timeout = timeout
	}
}

// WithAPIToken sets the authentication token for the HuggingFace Hub API
func WithAPIToken(token string) HuggingFaceHubClientOption {
	return func(c *HuggingFaceHubClientImpl) {
		c.apiToken = token
	}
}

// NewHuggingFaceHubClient creates a new HuggingFaceHubClient
func NewHuggingFaceHubClient(opts ...HuggingFaceHubClientOption) HuggingFaceHubClient {
	client := &HuggingFaceHubClientImpl{
		baseURL: defaultHuggingFaceHubAPIBaseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// GetModel fetches metadata for a specific model from HuggingFace Hub
func (c *HuggingFaceHubClientImpl) GetModel(ctx context.Context, owner, name string) (*HuggingFaceModel, error) {
	path := fmt.Sprintf("/models/%s/%s", url.PathEscape(owner), url.PathEscape(name))
	
	data, err := c.doRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var model HuggingFaceModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, Wrap(err, ErrInvalidResponse, "failed to parse model response")
	}
	
	model.RawResponse = data
	return &model, nil
}

// GetDataset fetches metadata for a specific dataset from HuggingFace Hub
func (c *HuggingFaceHubClientImpl) GetDataset(ctx context.Context, owner, name string) (*HuggingFaceDataset, error) {
	path := fmt.Sprintf("/datasets/%s/%s", url.PathEscape(owner), url.PathEscape(name))
	
	data, err := c.doRequest(ctx, path)
	if err != nil {
		return nil, err
	}

	var dataset HuggingFaceDataset
	if err := json.Unmarshal(data, &dataset); err != nil {
		return nil, Wrap(err, ErrInvalidResponse, "failed to parse dataset response")
	}
	
	dataset.RawResponse = data
	return &dataset, nil
}

// doRequest performs an HTTP request to the HuggingFace Hub API
func (c *HuggingFaceHubClientImpl) doRequest(ctx context.Context, path string) ([]byte, error) {
	url := c.baseURL + path
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, Wrap(err, ErrInvalidRequest, "failed to create request")
	}

	req.Header.Set("Accept", "application/json")
	
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, Wrap(err, ErrNetworkError, "failed to send request")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Wrap(err, ErrIOError, "failed to read response body")
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: HuggingFace Hub API error - HTTP %d: %s", 
			ErrAPIError, resp.StatusCode, string(data))
	}

	return data, nil
}