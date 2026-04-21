package aiservices

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateOpenAPISchemaForLLMResponse_FlatStruct(t *testing.T) {
	type flat struct {
		City    string `json:"city"`
		Country string `json:"country"`
	}

	schema, err := GenerateOpenAPISchemaForLLMResponse(&flat{})
	require.NoError(t, err)
	require.NotNil(t, schema)

	raw, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	assert.Equal(t, "object", m["type"])
	assert.Equal(t, false, m["additionalProperties"],
		"root object must have additionalProperties: false")

	props, ok := m["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "city")
	assert.Contains(t, props, "country")
}

func TestGenerateOpenAPISchemaForLLMResponse_NestedStruct(t *testing.T) {
	type address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	type person struct {
		Name    string  `json:"name"`
		Address address `json:"address"`
	}

	schema, err := GenerateOpenAPISchemaForLLMResponse(&person{})
	require.NoError(t, err)
	require.NotNil(t, schema)

	raw, err := json.Marshal(schema)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	assert.Equal(t, false, m["additionalProperties"],
		"root object must have additionalProperties: false")

	props := m["properties"].(map[string]any)
	addrSchema := props["address"].(map[string]any)

	assert.Equal(t, "object", addrSchema["type"],
		"nested address field must be an object")
	assert.Equal(t, false, addrSchema["additionalProperties"],
		"nested object must also have additionalProperties: false")
}

func TestGenerateOpenAPISchemaForLLMResponse_NilInput(t *testing.T) {
	_, err := GenerateOpenAPISchemaForLLMResponse(nil)
	assert.Error(t, err)
}
