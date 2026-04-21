package aiservices

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnthropicDirectAPI_GenerateSingle is an integration test that calls the real
// Anthropic API via the factory. It is skipped unless AISERVICES_ANTHROPIC_API_KEY is set.
//
// Run with:
//
//	AISERVICES_ANTHROPIC_API_KEY=sk-ant-... go test ./aiservices/... -run TestAnthropicDirectAPI_GenerateSingle -v
func TestAnthropicDirectAPI_GenerateSingle(t *testing.T) {
	apiKey := os.Getenv("AISERVICES_ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("skipping integration test: AISERVICES_ANTHROPIC_API_KEY is not set")
	}

	t.Setenv("AISERVICES_LLM_PROVIDER", "anthropic")
	t.Setenv("AISERVICES_ANTHROPIC_USE_BEDROCK", "false")
	t.Setenv("AISERVICES_ANTHROPIC_API_KEY", apiKey)
	t.Setenv("AISERVICES_ANTHROPIC_MAX_TOKENS", "1025") // our tests need less tokens (1024 is the min)

	provider, err := CreateLLMProviderFromEnv()
	require.NoError(t, err)
	require.NotNil(t, provider)

	model, err := provider.GetFastModel()
	require.NoError(t, err)
	require.NotNil(t, model)

	assert.Equal(t, Anthropic, model.GetProviderID())

	response, err := model.GenerateSingle(context.Background(), LLMGenerationRequest{
		SystemPrompt: "You are a helpful assistant. Answer concisely.",
		UserPrompt:   "What is 2 + 2 + 3? Reply with the number only.",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.True(t, strings.Contains(response, "7"),
		"expected response to contain '7', got: %q", response)

	thinkingModel, err := provider.GetReasoningModel()
	require.NoError(t, err)
	require.NotNil(t, thinkingModel)

	response, err = thinkingModel.GenerateSingle(context.Background(), LLMGenerationRequest{
		SystemPrompt: "You are a helpful assistant. Answer concisely.",
		UserPrompt:   "How many 'r's in the word strawberryrity",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.True(t, strings.Contains(response, "4"),
		"expected response to contain '4', got: %q", response)
}

// TestAnthropicDirectAPI_GenerateSingle_WithResponseSchema verifies that passing a
// ResponseSchema causes Claude to return valid JSON conforming to the schema.
//
// Run with:
//
//	AISERVICES_ANTHROPIC_API_KEY=sk-ant-... go test ./aiservices/... -run TestAnthropicDirectAPI_GenerateSingle_WithResponseSchema -v
func TestAnthropicDirectAPI_GenerateSingle_WithResponseSchema(t *testing.T) {
	apiKey := os.Getenv("AISERVICES_ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("skipping integration test: AISERVICES_ANTHROPIC_API_KEY is not set")
	}

	t.Setenv("AISERVICES_LLM_PROVIDER", "anthropic")
	t.Setenv("AISERVICES_ANTHROPIC_USE_BEDROCK", "false")
	t.Setenv("AISERVICES_ANTHROPIC_API_KEY", apiKey)
	t.Setenv("AISERVICES_ANTHROPIC_MAX_TOKENS", "1025")

	schema := openapi3.NewObjectSchema().
		WithProperty("city", openapi3.NewStringSchema()).
		WithProperty("country", openapi3.NewStringSchema()).
		WithoutAdditionalProperties()

	schema.Required = []string{"city", "country"}

	provider, err := CreateLLMProviderFromEnv(WithResponseSchema(schema))
	require.NoError(t, err)
	require.NotNil(t, provider)

	model, err := provider.GetFastModel() // also tested with reasoning model locally
	require.NoError(t, err)

	response, err := model.GenerateSingle(context.Background(), LLMGenerationRequest{
		SystemPrompt: "You are a helpful assistant",
		UserPrompt:   "What is the capital city of France? Return the city and country.",
	})

	require.NoError(t, err)
	require.NotEmpty(t, response)

	assert.Equal(t, `{"city": "Paris", "country": "France"}`, response)
}
