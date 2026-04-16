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
		name          string
		envVars       map[string]string
		opts          []LLMProviderBuilderOption
		expectError   bool
		strictSuccess bool // assert err == nil and provider != nil unconditionally
		errorCheck    func(t *testing.T, err error)
	}{
		// Vertex AI cases
		{
			name: "valid vertex ai config",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "google-vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			expectError: false,
		},
		{
			name: "valid vertex ai config with response schema",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "google-vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT":  "test-project",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			opts:        []LLMProviderBuilderOption{WithResponseSchema(&openapi3.Schema{Type: "object"})},
			expectError: false,
		},
		{
			name: "vertex ai missing project",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":              "google-vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_LOCATION": "us-central1",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name: "vertex ai missing location",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":             "google-vertex",
				"AISERVICES_GOOGLE_VERTEX_AI_PROJECT": "test-project",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		// Anthropic — Bedrock backend
		{
			name: "anthropic bedrock backend",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":          "anthropic",
				"AISERVICES_ANTHROPIC_USE_BEDROCK": "true",
				"AISERVICES_AWS_BEDROCK_REGION":    "us-east-1",
			},
			expectError:   false,
			strictSuccess: true,
		},
		{
			name: "anthropic bedrock backend missing region",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":          "anthropic",
				"AISERVICES_ANTHROPIC_USE_BEDROCK": "true",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_AWS_BEDROCK_REGION must be set")
			},
		},
		// Anthropic — direct API backend
		{
			name: "anthropic direct api backend",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":      "anthropic",
				"AISERVICES_ANTHROPIC_API_KEY": "test-api-key",
			},
			expectError:   false,
			strictSuccess: true,
		},
		{
			name: "anthropic direct api backend missing key",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER": "anthropic",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_ANTHROPIC_API_KEY must be set")
			},
		},
		{
			name: "anthropic with response schema returns error",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER":      "anthropic",
				"AISERVICES_ANTHROPIC_API_KEY": "test-api-key",
			},
			opts:        []LLMProviderBuilderOption{WithResponseSchema(&openapi3.Schema{Type: "object"})},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "WithResponseSchema is not supported for the Anthropic provider")
			},
		},
		// Unknown / empty provider — both fall back to Vertex AI; error comes from missing Vertex AI env vars.
		{
			name:        "empty provider falls back to vertex ai",
			envVars:     map[string]string{},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name: "unknown provider falls back to vertex ai",
			envVars: map[string]string{
				"AISERVICES_LLM_PROVIDER": "unknown",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			} else if tt.strictSuccess {
				require.NoError(t, err)
				assert.NotNil(t, provider)
			} else {
				// Provider construction may fail with credential errors in CI/CD.
				// We only assert on our own validation errors.
				if err != nil {
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
						"result": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}},
					},
				}),
			},
			expectError: false,
		},
		{
			name:        "empty project",
			location:    "us-central1",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
		{
			name:        "empty location",
			project:     "test-project",
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_GOOGLE_VERTEX_AI_PROJECT and AISERVICES_GOOGLE_VERTEX_AI_LOCATION must be set")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					assert.Contains(t, err.Error(), "failed to load credentials")
				}
			}
		})
	}
}

func TestCreateAnthropicProvider(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		opts        []LLMProviderBuilderOption
		expectError bool
		errorCheck  func(t *testing.T, err error)
	}{
		// Bedrock backend
		{
			name: "bedrock backend valid",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_USE_BEDROCK": "true",
				"AISERVICES_AWS_BEDROCK_REGION":    "us-east-1",
			},
			expectError: false,
		},
		{
			name: "bedrock backend missing region",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_USE_BEDROCK": "true",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_AWS_BEDROCK_REGION must be set")
			},
		},
		// Direct API backend
		{
			name: "direct api backend valid",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY": "test-key",
			},
			expectError: false,
		},
		{
			name:        "direct api backend missing key",
			envVars:     map[string]string{},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "AISERVICES_ANTHROPIC_API_KEY must be set")
			},
		},
		// Shared
		{
			name: "response schema not supported",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY": "test-key",
			},
			opts:        []LLMProviderBuilderOption{WithResponseSchema(&openapi3.Schema{Type: "object"})},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "WithResponseSchema is not supported for the Anthropic provider")
			},
		},
		// Optional tuning env vars
		{
			name: "max tokens from env",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY":    "test-key",
				"AISERVICES_ANTHROPIC_MAX_TOKENS": "4096",
			},
			expectError: false,
		},
		{
			name: "invalid max tokens from env returns error",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY":    "test-key",
				"AISERVICES_ANTHROPIC_MAX_TOKENS": "not-a-number",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid value for AISERVICES_ANTHROPIC_MAX_TOKENS")
			},
		},
		{
			name: "thinking budget tokens from env",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY":                "test-key",
				"AISERVICES_ANTHROPIC_THINKING_BUDGET_TOKENS": "2048",
			},
			expectError: false,
		},
		{
			name: "invalid thinking budget tokens from env returns error",
			envVars: map[string]string{
				"AISERVICES_ANTHROPIC_API_KEY":                "test-key",
				"AISERVICES_ANTHROPIC_THINKING_BUDGET_TOKENS": "not-a-number",
			},
			expectError: true,
			errorCheck: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "invalid value for AISERVICES_ANTHROPIC_THINKING_BUDGET_TOKENS")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			provider, err := createAnthropicProvider(tt.opts...)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, provider)
				if tt.errorCheck != nil {
					tt.errorCheck(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
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
			name:     "no options",
			opts:     nil,
			expected: &llmProviderBuilderOptions{responseSchema: nil},
		},
		{
			name:     "empty options slice",
			opts:     []LLMProviderBuilderOption{},
			expected: &llmProviderBuilderOptions{responseSchema: nil},
		},
		{
			name: "with response schema",
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"name": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}},
					},
				}),
			},
			expected: &llmProviderBuilderOptions{
				responseSchema: &openapi3.Schema{
					Type: "object",
					Properties: openapi3.Schemas{
						"name": &openapi3.SchemaRef{Value: &openapi3.Schema{Type: "string"}},
					},
				},
			},
		},
		{
			name: "multiple options last one wins",
			opts: []LLMProviderBuilderOption{
				WithResponseSchema(&openapi3.Schema{Type: "string"}),
				WithResponseSchema(&openapi3.Schema{Type: "object"}),
			},
			expected: &llmProviderBuilderOptions{responseSchema: &openapi3.Schema{Type: "object"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builderOptionsFromOpts(tt.opts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
