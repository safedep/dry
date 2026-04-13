package aiservices

import (
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	aiServicesLLMProviderGoogleVertexAI = string(GoogleVertex)
	aiServicesLLMProviderGoogleGemini   = string(Google)
	aiServicesLLMProviderOpenAI         = string(OpenAI)
	aiServicesLLMProviderAnthropic      = string(Anthropic)
)

type llmProviderBuilderOptions struct {
	// Some models support a response schema
	responseSchema *openapi3.Schema
}

type LLMProviderBuilderOption func(*llmProviderBuilderOptions)

// WithResponseSchema sets the response schema for the provider.
func WithResponseSchema(schema *openapi3.Schema) LLMProviderBuilderOption {
	return func(opts *llmProviderBuilderOptions) {
		opts.responseSchema = schema
	}
}

// CreateLLMProviderFromEnv creates an LLMProvider based on environment variables.
//
// Required env vars:
//
//	AISERVICES_LLM_PROVIDER — one of: google-vertex, anthropic
//
// For "anthropic":
//
//	 For AWS Bedrock:
//
//		AISERVICES_ANTHROPIC_USE_BEDROCK=true   — use AWS Bedrock backend
//		AISERVICES_AWS_BEDROCK_REGION         — required for Bedrock
//		AISERVICES_AWS_BEDROCK_ACCESS_KEY     — optional (falls back to credential chain)
//		AISERVICES_AWS_BEDROCK_SECRET_ACCESS_KEY
//		AISERVICES_AWS_BEDROCK_SESSION_TOKEN
//		AISERVICES_AWS_BEDROCK_PROFILE
//
//	 For Anthropic Cloud:
//
//		AISERVICES_ANTHROPIC_USE_BEDROCK=false  — use direct Anthropic API backend (default)
//		AISERVICES_ANTHROPIC_API_KEY          — required for direct API
//		AISERVICES_ANTHROPIC_BASE_URL         — optional custom endpoint
func CreateLLMProviderFromEnv(opts ...LLMProviderBuilderOption) (LLMProvider, error) {
	providerType := strings.ToLower(strings.TrimSpace(os.Getenv("AISERVICES_LLM_PROVIDER")))
	switch providerType {
	case aiServicesLLMProviderGoogleVertexAI:
		return createVertexAIProvider(opts...)
	case aiServicesLLMProviderAnthropic:
		return createAnthropicProvider(opts...)
	case aiServicesLLMProviderOpenAI:
		return nil, fmt.Errorf("provider not supported: OpenAI")
	case aiServicesLLMProviderGoogleGemini:
		return nil, fmt.Errorf("provider not supported: Google")
	default:
		return nil, fmt.Errorf("unknown provider %q: set AISERVICES_LLM_PROVIDER to one of: %s, %s",
			providerType, aiServicesLLMProviderGoogleVertexAI, aiServicesLLMProviderAnthropic)
	}
}

func createVertexAIProvider(builderOpts ...LLMProviderBuilderOption) (LLMProvider, error) {
	project := os.Getenv("AISERVICES_GOOGLE_VERTEX_AI_PROJECT")
	location := os.Getenv("AISERVICES_GOOGLE_VERTEX_AI_LOCATION")

	if project == "" || location == "" {
		return nil, fmt.Errorf("missing required environment variables for Google Vertex AI: " +
			"AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
	}

	// This can be empty. Google SDK automatically picks up the default credentials.
	credentialsFile := os.Getenv("AISERVICES_GOOGLE_VERTEX_AI_CREDENTIALS_FILE")

	config := VertexAIModelConfig{
		Project:         project,
		Location:        location,
		CredentialsFile: credentialsFile,
	}

	builderOptions := builderOptionsFromOpts(builderOpts...)
	if builderOptions.responseSchema != nil {
		config.ResponseSchema = builderOptions.responseSchema
	}

	return NewGoogleVertexAIModelProvider(config)
}

func createAnthropicProvider(builderOpts ...LLMProviderBuilderOption) (LLMProvider, error) {
	opts := builderOptionsFromOpts(builderOpts...)
	if opts.responseSchema != nil {
		return nil, fmt.Errorf("WithResponseSchema is not supported for the Anthropic provider")
	}

	config := AnthropicModelConfig{
		UseBedrock: strings.EqualFold(os.Getenv("AISERVICES_ANTHROPIC_USE_BEDROCK"), "true"),
	}

	if config.UseBedrock {
		config.Region = os.Getenv("AISERVICES_AWS_BEDROCK_REGION")
		if config.Region == "" {
			return nil, fmt.Errorf("missing required environment variable for Anthropic Bedrock backend: " +
				"AISERVICES_AWS_BEDROCK_REGION must be set")
		}
		// Optional explicit credentials. When empty the AWS default credential chain is used.
		config.AccessKey = os.Getenv("AISERVICES_AWS_BEDROCK_ACCESS_KEY")
		config.SecretAccessKey = os.Getenv("AISERVICES_AWS_BEDROCK_SECRET_ACCESS_KEY")
		config.SessionToken = os.Getenv("AISERVICES_AWS_BEDROCK_SESSION_TOKEN")
		config.Profile = os.Getenv("AISERVICES_AWS_BEDROCK_PROFILE")
	} else {
		config.APIKey = os.Getenv("AISERVICES_ANTHROPIC_API_KEY")
		if config.APIKey == "" {
			return nil, fmt.Errorf("missing required environment variable for Anthropic API backend: " +
				"AISERVICES_ANTHROPIC_API_KEY must be set")
		}
		if baseURL := os.Getenv("AISERVICES_ANTHROPIC_BASE_URL"); baseURL != "" {
			config.BaseURL = &baseURL
		}
	}

	return NewAnthropicModelProvider(config)
}

func builderOptionsFromOpts(opts ...LLMProviderBuilderOption) *llmProviderBuilderOptions {
	builderOpts := &llmProviderBuilderOptions{}
	for _, opt := range opts {
		opt(builderOpts)
	}

	return builderOpts
}
