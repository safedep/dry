package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAsApiError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		isOk bool
	}{
		{
			"Unwrappable error",
			stderrors.New("err"),
			false,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err, ok := AsApiError(test.err)
			assert.NotNil(t, err)
			assert.Equal(t, test.isOk, ok)
		})
	}
}

func TestApiErrorBuilder(t *testing.T) {
	err := BuildApiError("200", "Test error", "test_type")
	assert.NotNil(t, err)

	assert.Equal(t, "200", *err.ApiError().Code)
	assert.Equal(t, "Test error", *err.ApiError().Message)
	assert.Equal(t, "test_type", *err.ApiError().Type)
}
