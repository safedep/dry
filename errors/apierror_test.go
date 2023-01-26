package errors

import (
	stderrors "errors"
	"testing"

	"github.com/safedep/dry/api"
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
		{
			"Wrappable error",
			BuildApiError(api.ApiErrorTypeInvalidRequest,
				api.ApiErrorCodeAppGenericError, "Test"),
			true,
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
	err := BuildApiError("test_type", "200", "Test error")
	assert.NotNil(t, err)

	assert.Equal(t, api.ApiErrorCode("200"), *err.ApiError().Code)
	assert.Equal(t, api.ApiErrorType("test_type"), *err.ApiError().Type)
	assert.Equal(t, "Test error", *err.ApiError().Message)
}

func TestApiErrorAddParams(t *testing.T) {
	err := BuildApiError("test_type", "200", "Test error")
	err.AddParam("p1", "v1")

	assert.NotNil(t, err.apiErr.Params)

	v, ok := err.apiErr.Params.Get("p1")
	assert.True(t, ok)
	assert.Equal(t, "v1", *v.Value)

	_, ok = err.apiErr.Params.Get("p2")
	assert.False(t, ok)
}
