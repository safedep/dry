package apiguard

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"
	swagger "github.com/safedep/dry/apiguard/tykgen"
)

// Contract for a management client
type ManagementClient interface {
	// Create a key in the API Guard. We will also generate a custom key and
	// not depend on API Guard's key generation.
	CreateKey(context.Context, KeyArgs) (ApiKey, error)
}

type managementClient struct {
	baseUrl   string
	token     string
	client    http.Client
	tykClient *swagger.APIClient
	keyGen    KeyGen
}

type ManagementClientOpts func(*managementClient)

// NewManagementClient creates a new management client for the API Guard.
func NewManagementClient(baseUrl, token string, opts ...ManagementClientOpts) (ManagementClient, error) {
	if len(baseUrl) > 0 && baseUrl[len(baseUrl)-1] == '/' {
		baseUrl = baseUrl[:len(baseUrl)-1]
	}

	client := &managementClient{
		token:   token,
		keyGen:  defaultKeyGen(),
		baseUrl: baseUrl,
		client:  http.Client{},
	}

	for _, opt := range opts {
		opt(client)
	}

	tykClientConfig := swagger.NewConfiguration()
	tykClientConfig.BasePath = baseUrl
	tykClientConfig.AddDefaultHeader("x-tyk-authorization", token)
	tykClientConfig.HTTPClient = &client.client

	client.tykClient = swagger.NewAPIClient(tykClientConfig)
	return client, nil
}

func WithHTTPClient(httpClient http.Client) ManagementClientOpts {
	return func(c *managementClient) {
		c.client = httpClient
	}
}

func WithKeyGen(keyGen KeyGen) ManagementClientOpts {
	return func(c *managementClient) {
		c.keyGen = keyGen
	}
}

func (c *managementClient) CreateKey(ctx context.Context, args KeyArgs) (ApiKey, error) {
	sessionState := swagger.SessionState{
		Tags:          args.Tags,
		Alias:         args.Alias,
		ApplyPolicyId: args.PolicyId,
		ApplyPolicies: args.Policies,
		Expires:       args.ExpiresAt.Unix(),
		MetaData: map[string]interface{}{
			"org_id":  args.Info.OrganizationID,
			"team_id": args.Info.TeamID,
			"user_id": args.Info.UserID,
			"key_id":  args.Info.KeyID,
		},
	}

	key, err := c.keyGen()
	if err != nil {
		return ApiKey{}, fmt.Errorf("failed to generate key: %w", err)
	}

	apiRes, res, err := c.tykClient.KeysApi.CreateCustomKey(ctx, key, &swagger.KeysApiCreateCustomKeyOpts{
		Body: optional.NewInterface(&sessionState),
	})

	if err != nil {
		return ApiKey{}, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ApiKey{}, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	if apiRes.Key == "" {
		return ApiKey{}, fmt.Errorf("API Guard did not return a key")
	}

	return ApiKey{
		Key:       apiRes.Key,
		KeyId:     apiRes.KeyHash,
		ExpiresAt: args.ExpiresAt,
	}, nil
}
