package aiservices

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicProvider_Bedrock(t *testing.T) {
	config := AnthropicModelConfig{
		UseBedrock: true,
		Region:     "us-east-1",
	}

	provider, err := NewAnthropicModelProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, Anthropic, provider.GetID())

	testCases := []struct {
		name         string
		expectErr    bool
		errCheckFunc func(error) bool
		setupModel   func() (Model, error)
	}{
		{
			name: "FastModel uses Bedrock model ID",
			setupModel: func() (Model, error) {
				return provider.GetFastModel()
			},
		},
		{
			name: "ReasoningModel uses Bedrock model ID",
			setupModel: func() (Model, error) {
				return provider.GetReasoningModel()
			},
		},
		{
			name: "GetModelByID",
			setupModel: func() (Model, error) {
				return provider.GetModelByID("anthropic.claude-3-haiku-20240307-v1:0")
			},
		},
		{
			name:         "EmptyModelID returns InvalidConfigError",
			expectErr:    true,
			errCheckFunc: IsInvalidConfigError,
			setupModel: func() (Model, error) {
				return provider.GetModelByID("")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := tc.setupModel()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, model)
				if tc.errCheckFunc != nil {
					assert.True(t, tc.errCheckFunc(err), "expected specific error type, got: %v", err)
				}
			} else {
				// Model creation may fail with auth errors in environments without AWS credentials.
				// Only failures originating from our validation code are unexpected here.
				if err != nil {
					assert.True(t, IsAuthenticationError(err), "unexpected error type: %v", err)
				} else {
					assert.NotNil(t, model)
					assert.Equal(t, Anthropic, model.GetProviderID())
				}
			}
		})
	}
}

func TestAnthropicProvider_DirectAPI(t *testing.T) {
	config := AnthropicModelConfig{
		UseBedrock: false,
		APIKey:     "test-api-key",
	}

	provider, err := NewAnthropicModelProvider(config)
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, Anthropic, provider.GetID())

	testCases := []struct {
		name         string
		expectErr    bool
		errCheckFunc func(error) bool
		setupModel   func() (Model, error)
	}{
		{
			name: "FastModel uses direct API model ID",
			setupModel: func() (Model, error) {
				return provider.GetFastModel()
			},
		},
		{
			name: "ReasoningModel uses direct API model ID",
			setupModel: func() (Model, error) {
				return provider.GetReasoningModel()
			},
		},
		{
			name:         "EmptyModelID returns InvalidConfigError",
			expectErr:    true,
			errCheckFunc: IsInvalidConfigError,
			setupModel: func() (Model, error) {
				return provider.GetModelByID("")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := tc.setupModel()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, model)
				if tc.errCheckFunc != nil {
					assert.True(t, tc.errCheckFunc(err), "expected specific error type, got: %v", err)
				}
			} else {
				// Model creation may fail with auth errors in test environments.
				if err != nil {
					assert.True(t, IsAuthenticationError(err), "unexpected error type: %v", err)
				} else {
					assert.NotNil(t, model)
					assert.Equal(t, Anthropic, model.GetProviderID())
				}
			}
		})
	}
}

func TestAnthropicProvider_BedrockMissingRegion(t *testing.T) {
	provider, err := NewAnthropicModelProvider(AnthropicModelConfig{UseBedrock: true, Region: ""})
	require.NoError(t, err)

	model, err := provider.GetFastModel()
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.True(t, IsInvalidConfigError(err), "expected invalid config error for missing region, got: %v", err)
}

func TestAnthropicProvider_DirectAPIMissingKey(t *testing.T) {
	provider, err := NewAnthropicModelProvider(AnthropicModelConfig{UseBedrock: false, APIKey: ""})
	require.NoError(t, err)

	model, err := provider.GetFastModel()
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.True(t, IsInvalidConfigError(err), "expected invalid config error for missing API key, got: %v", err)
}
