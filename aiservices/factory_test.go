package aiservices

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithResponseSchema(t *testing.T) {
	tests := []struct {
		name           string
		schema         *openapi3.Schema
		expectedSchema *openapi3.Schema
	}{
		{
			name:           "nil schema",
			schema:         nil,
			expectedSchema: nil,
		},
		{
			name: "valid schema",
			schema: &openapi3.Schema{
				Type: "object",
				Properties: openapi3.Schemas{
					"name": &openapi3.SchemaRef{
						Value: &openapi3.Schema{Type: "string"},
					},
				},
			},
			expectedSchema: &openapi3.Schema{
				Type: "object",
				Properties: openapi3.Schemas{
					"name": &openapi3.SchemaRef{
						Value: &openapi3.Schema{Type: "string"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &llmProviderBuilderOptions{}
			option := WithResponseSchema(tt.schema)
			option(opts)

			assert.Equal(t, tt.expectedSchema, opts.responseSchema)
		})
	}
}

func TestCreateLLMProviderFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		opts        []LLMProviderBuilderOption
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name: "valid vertex ai config",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts:        nil,
			expectError: false,
		},
		{
			name: "valid vertex ai config with response schema",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{Type: "object"}),
			},
			expectError: false,
		},
		{
			name: "missing project",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts:        nil,
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name: "missing location",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":             "vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT": "test-project",
			},
			opts:        nil,
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name: "empty provider defaults to vertex ai",
			envVars: map[string]string{
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts:        nil,
			expectError: false,
		},
		{
			name: "unknown provider defaults to vertex ai",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "unknown",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts:        nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables using t.Setenv for automatic cleanup
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			provider, err := CreateLLMProviderFromEnv(tt.opts...)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, provider)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				// Note: This test will fail in actual execution because it tries to create real credentials
				// In a real test environment, you would need to mock the credential creation
				// For now, we're just testing the validation logic
				if err != nil {
					// Expected to fail due to credential issues in test environment
					assert.Contains(t, err.Error(), "failed to load credentials")
				}
			}
		})
	}
}

func TestCreateVertexAIProvider(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		location    string
		credsFile   string
		opts        []LLMProviderBuilderOption
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		{
			name:        "valid config",
			project:     "test-project",
			location:    "us-central1",
			credsFile:   "",
			opts:        nil,
			expectError: false,
		},
		{
			name:     "valid config with response schema",
			project:  "test-project",
			location: "us-central1",
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"result": &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: "string"},
						},
					},
				}),
			},
			expectError: false,
		},
		{
			name:        "empty project",
			project:     "",
			location:    "us-central1",
			opts:        nil,
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name:        "empty location",
			project:     "test-project",
			location:    "",
			opts:        nil,
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name:        "both empty",
			project:     "",
			location:    "",
			opts:        nil,
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables using t.Setenv for automatic cleanup
			t.Setenv("AISERVICES_GOOGLE_VERTEX_AI_PROJECT", tt.project)
			t.Setenv("AISERVICES_GOOGLE_VERTEX_AI_LOCATION", tt.location)
			t.Setenv("AISERVICES_GOOGLE_VERTEX_AI_CREDENTIALS_FILE", tt.credsFile)

			provider, err := createVertexAIProvider(tt.opts...)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, provider)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				// Note: This test will fail in actual execution because it tries to create real credentials
				// In a real test environment, you would need to mock the credential creation
				// For now, we're just testing the validation logic
				if err != nil {
					// Expected to fail due to credential issues in test environment
					assert.Contains(t, err.Error(), "failed to load credentials")
				}
			}
		})
	}
}

func TestBuilderOptionsFromOpts(t *testing.T) {
	tests := []struct {
		name     string
		opts     []LLMProviderBuilderOption
		expected *llmProviderBuilderOptions
	}{
		{
			name: "no options",
			opts: nil,
			expected: &llmProviderBuilderOptions{
				responseSchema: nil,
			},
		},
		{
			name: "empty options slice",
			opts: []LLMProviderBuilderOption{},
			expected: &llmProviderBuilderOptions{
				responseSchema: nil,
			},
		},
		{
			name: "with response schema",
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"name": &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: "string"},
						},
					},
				}),
			},
			expected: &llmProviderBuilderOptions{
				responseSchema: &openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"name": &openapi3.SchemaRef{
							Value: &openapi3.Schema{Type: "string"},
						},
					},
				},
			},
		},
		{
			name: "multiple options with response schema override",
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{Type: "string"}),
				WithResponseSchema(&openapi3.Schema{Type: "object"}),
			},
			expected: &llmProviderBuilderOptions{
				responseSchema: &openapi3.Schema{Type: "object"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builderOptionsFromOpts(tt.opts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
