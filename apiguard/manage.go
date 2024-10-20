package apiguard

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"

	"github.com/antihax/optional"
	swagger "github.com/safedep/dry/apiguard/tykgen"
)

// Contract for a management client
type ManagementClient interface {
	// Create a key in the API Guard. We will also generate a custom key and
	// not depend on API Guard's key generation.
	CreateKey(context.Context, KeyArgs) (ApiKey, error)

	// List policies in the API Guard
	ListPolicies(context.Context) ([]Policy, error)

	// Delete an API key by key hash
	DeleteKey(context.Context, string) error
}

type managementClient struct {
	baseUrl   string
	token     string
	client    http.Client
	tykClient *swagger.APIClient
	keyGen    KeyGen
}

type ManagementClientOpts func(*managementClient)

// Helper to standardize the creation of a management client from environment
// based configuration
func NewManagementClientFromEnvConfig() (ManagementClient, error) {
	baseUrl := os.Getenv("APIGUARD_BASE_URL")
	if baseUrl == "" {
		return nil, fmt.Errorf("APIGUARD_BASE_URL is not set")
	}

	token := os.Getenv("APIGUARD_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("APIGUARD_TOKEN is not set")
	}

	skipTlsVerify := os.Getenv("INSECURE_APIGUARD_SKIP_TLS_VERIFY") == "true"

	httpClient := http.Client{}
	if skipTlsVerify {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTlsVerify},
		}
	}

	client, err := NewManagementClient(baseUrl, token,
		WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("Failed to create management client: %v", err)
	}

	return client, nil
}

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

func (c *managementClient) DeleteKey(ctx context.Context, keyHash string) error {
	_, res, err := c.tykClient.KeysApi.DeleteKey(ctx, keyHash,
		&swagger.KeysApiDeleteKeyOpts{Hashed: optional.NewBool(true)})
	if err != nil {
		return err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (c *managementClient) ListPolicies(ctx context.Context) ([]Policy, error) {
	policies, res, err := c.tykClient.PoliciesApi.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var result []Policy
	for _, p := range policies {
		policy := Policy{
			InternalID:         p.InternalId,
			ID:                 p.Id,
			Name:               p.Name,
			QuotaMax:           p.QuotaMax,
			QuotaRenewalRate:   p.QuotaRenewalRate,
			Rate:               p.Rate,
			RateInterval:       p.Per,
			ThrottleInterval:   p.ThrottleInterval,
			ThrottleRetryLimit: p.ThrottleRetryLimit,
			Active:             p.Active,
			AccessRights:       make([]PolicyAccess, 0, len(p.AccessRights)),
		}

		for apiID, access := range p.AccessRights {
			policy.AccessRights = append(policy.AccessRights, PolicyAccess{
				ApiID:   apiID,
				ApiName: access.ApiName,
			})
		}

		result = append(result, policy)
	}

	return result, nil
}
