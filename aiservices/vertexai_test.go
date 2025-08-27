package aiservices

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewVertexAIModel(t *testing.T) {
	config := VertexAIModelConfig{
		Project:         "text-project",
		Location:        "us-central1",
		CredentialsFile: "./testdata/vertexai_cred.json",
	}

	testCases := []struct {
		name     string
		modelId  string
		expected string
	}{
		{
			name:     "Fast Model",
			modelId:  vertexAIFastModelId,
			expected: vertexAIFastModelId,
		},
		{
			name:     "Reasoning Model",
			modelId:  vertexAIReasoningModelId,
			expected: vertexAIReasoningModelId,
		},
		{
			name:     "Custom Gemma Model",
			modelId:  "gemma-2b-it-lora",
			expected: "gemma-2b-it-lora",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := newVertexAIChatModel(t.Context(), tc.modelId, config)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			assert.NotNil(t, model)
			assert.IsType(t, &googleVertexAIModel{}, model)
			assert.Equal(t, tc.expected, model.GetId())
			assert.Equal(t, GoogleVertex, model.GetProvider())
		})
	}
}

func TestNewVertexAI_AuthenticationError(t *testing.T) {
	config := VertexAIModelConfig{
		Project:         "text-project",
		Location:        "us-central1",
		CredentialsFile: "./testdata/invalid_cred.json", // Invalid credentials file
	}

	model, err := newVertexAIChatModel(t.Context(), vertexAIFastModelId, config)
	if err == nil {
		t.Fatal("expected an error due to invalid credentials, got nil")
	}

	assert.Nil(t, model)
	assert.True(t, IsAuthenticationError(err), "expected an authentication error")
}