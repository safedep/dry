package aiservices

import (
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/pkg/errors"
)

// GenerateOpenAPISchemaForLLMResponse converts a Go struct into an OpenAPI Schema.
// Since its open standard, we are not providing it for clients, they need to handle it.
//
// "additionalProperties: false", is a JSON schema keyword that restricts objects to only allow properties defined in the schema.
// This is set on every object schema during generation to ensure strict validation.
// This is required by Anthropic's structured output API and is harmless for other providers.
func GenerateOpenAPISchemaForLLMResponse(T any) (*openapi3.Schema, error) {
	schemas := make(openapi3.Schemas)

	schemaRef, err := openapi3gen.NewSchemaRefForValue(T, schemas,
		openapi3gen.SchemaCustomizer(func(_ string, _ reflect.Type, _ reflect.StructTag, schema *openapi3.Schema) error {
			if schema.Type == "object" {
				schema.WithoutAdditionalProperties()
			}
			return nil
		}),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create schema ref")
	}

	return schemaRef.Value, nil
}
