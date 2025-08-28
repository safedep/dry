package aiservices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVertexAIProvider(t *testing.T) {
	config := VertexAIModelConfig{
		Project:        "text-project",
		Location:       "us-central1",
		CredentialsFile: "./testdata/vertexai_cred.json",
	}

	provider, err := NewGoogleVertexAIModelProvider(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)

	testCases := []struct {
		name         string
		modelID      string
		expectedID   string
		expectErr    bool
		errCheckFunc func(error) bool
		setupModel   func() (Model, error)
	}{
		{
			name:       "FastModel",
			modelID:    vertexAIFastModelId,
			expectedID: vertexAIFastModelId,
			setupModel: func() (Model, error) {
				return provider.GetFastModel()
			},
		},
		{
			name:       "ReasoningModel",
			modelID:    vertexAIReasoningModelId,
			expectedID: vertexAIReasoningModelId,
			setupModel: func() (Model, error) {
				return provider.GetReasoningModel()
			},
		},
		{
			name:       "CustomModel",
			modelID:    "gemma-2b-it-lora",
			expectedID: "gemma-2b-it-lora",
			setupModel: func() (Model, error) {
				return provider.GetModelByID("gemma-2b-it-lora")
			},
		},
		{
			name:         "InvalidModel",
			modelID:      "",
			expectedID:   "",
			expectErr:    true,
			errCheckFunc: IsInvalidConfigError,
			setupModel: func() (Model, error) {
				return provider.GetModelByID("")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var model Model
			var err error

			model, err = tc.setupModel()

			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, model)
				if tc.errCheckFunc != nil {
					assert.True(t, tc.errCheckFunc(err), "expected specific error type")
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, model)
				assert.Equal(t, tc.expectedID, model.GetId())
			}
		})
	}
}

func TestVertexAIProviderAuthError(t *testing.T) {
	config := VertexAIModelConfig{
		Project:        "text-project",
		Location:       "us-central1",
		CredentialsFile: "./testdata/invalid_cred.json",
	}

	provider, err := NewGoogleVertexAIModelProvider(config)
	assert.NoError(t, err)
	assert.NotNil(t, provider)

	model, err := provider.GetFastModel()
	assert.Error(t, err)
	assert.Nil(t, model)
	assert.True(t, IsAuthenticationError(err), "expected authentication error")
}