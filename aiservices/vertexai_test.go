package aiservices

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/pkg/errors"
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
			model, err := newVertexAIChatModel(tc.modelId, config)
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

	model, err := newVertexAIChatModel(vertexAIFastModelId, config)
	if err == nil {
		t.Fatal("expected an error due to invalid credentials, got nil")
	}

	assert.Nil(t, model)
	assert.True(t, IsAuthenticationError(err), "expected an authentication error")
}

func TestVertexAI_CustomResponseSchema(t *testing.T) {
	type testCustomResponseSchema struct {
		Answer string `json:"answer"`
		Error  string `json:"error"`
	}

	schema, err := generateOpenapiSchema(&testCustomResponseSchema{})
	assert.NoError(t, err)

	config := VertexAIModelConfig{
		Project:         "text-project",
		Location:        "us-central1",
		CredentialsFile: "./testdata/vertexai_cred.json",
		ResponseSchema:  schema,
	}

	model, err := newVertexAIChatModel(vertexAIFastModelId, config)
	assert.NoError(t, err)

	assert.NotNil(t, model)
}

// generateOpenapiSchema helper function to convert Go struct into OpenAPI Schema
// Since its open standard, we are not providing it for clients, they need to handle it.
func generateOpenapiSchema(T any) (*openapi3.Schema, error) {
	schemas := make(openapi3.Schemas)

	schemaRef, err := openapi3gen.NewSchemaRefForValue(T, schemas)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create schema ref")
	}

	return schemaRef.Value, nil
}
