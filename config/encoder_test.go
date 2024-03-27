package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type simpleStruct struct {
	Name string
}

func TestConfigJSONEncoder(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{
			name:  "string",
			value: "hello",
		},
		{
			// Default type for numbers is float64
			name:  "float64",
			value: float64(42),
		},
		{
			name:  "bool/true",
			value: true,
		},
		{
			name:  "bool/false",
			value: false,
		},
	}

	encoder := &JSONConfigEncoder[any]{}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := encoder.Encode(test.value)
			assert.NoError(t, err)

			decoded, err := encoder.Decode(encoded)
			assert.NoError(t, err)

			assert.Equal(t, test.value, decoded)
		})
	}
}

func TestConfigJSONEncoderStruct(t *testing.T) {
	v := simpleStruct{Name: "hello"}
	encoder := &JSONConfigEncoder[simpleStruct]{}

	encoded, err := encoder.Encode(v)
	assert.NoError(t, err)

	decoded, err := encoder.Decode(encoded)
	assert.NoError(t, err)

	assert.Equal(t, v, decoded)
}
