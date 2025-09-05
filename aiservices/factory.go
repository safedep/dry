package aiservices

import (
	"fmt"
	"os"

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

// CreateProviderFromEnv creates an LLMProvider based on environment variables.
// It returns an error if the provider cannot be created.
func CreateLLMProviderFromEnv(opts ...LLMProviderBuilderOption) (LLMProvider, error) {
	providerType := os.Getenv("AISERVICES_LLM_PROVIDER")
	switch providerType {
	case aiServicesLLMProviderGoogleVertexAI:
		return createVertexAIProvider(opts...)
	case aiServicesLLMProviderOpenAI:
		return nil, fmt.Errorf("provider not supported: OpenAI")
	case aiServicesLLMProviderAnthropic:
		return nil, fmt.Errorf("provider not supported: Anthropic")
	case aiServicesLLMProviderGoogleGemini:
		return nil, fmt.Errorf("provider not supported: Google")
	default:
		// We only support Vertex AI for now.
		// But this is where we would add support for other providers in the future.
		return createVertexAIProvider(opts...)
	}
}

func createVertexAIProvider(builderOpts ...LLMProviderBuilderOption) (LLMProvider, error) {
	project := os.Getenv("AISERVICES_GOOGLE_VERTEX_AI_PROJECT")
	location := os.Getenv("AISERVICES_GOOGLE_VERTEX_AI_LOCATION")

	if project == "" || location == "" {
		return nil, fmt.Errorf("GOOGLE_VERTEX_AI_PROJECT and GOOGLE_VERTEX_AI_LOCATION must be set for Vertex AI provider")
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

func builderOptionsFromOpts(opts ...LLMProviderBuilderOption) *llmProviderBuilderOptions {
	builderOpts := &llmProviderBuilderOptions{}
	for _, opt := range opts {
		opt(builderOpts)
	}

	return builderOpts
}
