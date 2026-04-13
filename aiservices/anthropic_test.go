package aiservices

import (
	"context"
	"os"
	"strings"
	"testing"

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
		UserPrompt:   "What is the capital of India, reply with the city name only. in Capital Letters",
	})

	require.NoError(t, err)
	assert.NotEmpty(t, response)
	assert.True(t, strings.Contains(response, "NEW DELHI"),
		"expected response to contain 'NEW DELHI', got: %q", response)
}
