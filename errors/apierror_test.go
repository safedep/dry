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

func TestUnmarshalApiError(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		ok      bool
		errType string
		code    string
	}{
		{
			"Valid error JSON",
			`
			{
				"code": "api_guard_invalid_credentials",
				"message": "Key has expired, please renew",
				"type": "invalid_request"
			}
			`,
			true,
			"invalid_request",
			"api_guard_invalid_credentials",
		},
		{
			"Valid JSON but schema mismatch",
			`
			{
				"a": "b"
			}
			`,
			false,
			string(api.ApiErrorTypeInternalError),
			string(api.ApiErrorCodeAppGenericError),
		},
		{
			"Invalid JSON",
			`
			NOT A JSON
			`,
			false,
			string(api.ApiErrorTypeInternalError),
			string(api.ApiErrorCodeAppGenericError),
		},
	}

	for _, test := range cases {
		apiErr, ok := UnmarshalApiError([]byte(test.body))
		if test.ok {
			assert.True(t, ok)
			assert.Equal(t, test.errType, string(*apiErr.apiErr.Type))
			assert.Equal(t, test.code, string(*apiErr.apiErr.Code))
		} else {
			assert.False(t, ok)
			assert.Equal(t, string(api.ApiErrorTypeInternalError),
				string(*apiErr.apiErr.Type))
		}
	}
}

func TestApiErrorErrorMessage(t *testing.T) {
	errJson := `
			{
				"code": "api_guard_invalid_credentials",
				"message": "Key has expired, please renew",
				"type": "invalid_request",
				"params": {
					"key1": {
						"key": "key1",
						"value": "value1"
					}
				}
			}
			`
	apiErr, ok := UnmarshalApiError([]byte(errJson))
	assert.True(t, ok)
	assert.ErrorContains(t, apiErr, "Code=api_guard_invalid_credentials")
	assert.ErrorContains(t, apiErr, "Type=invalid_request")
	assert.ErrorContains(t, apiErr, "Params=[ key1:\"value1\" ]")
}

func TestApiErrorErrorMessageWithoutParams(t *testing.T) {
	errJson := `
			{
				"code": "api_guard_invalid_credentials",
				"message": "Key has expired, please renew",
				"type": "invalid_request"
			}
			`
	apiErr, ok := UnmarshalApiError([]byte(errJson))
	assert.True(t, ok)
	assert.ErrorContains(t, apiErr, "Code=api_guard_invalid_credentials")
	assert.ErrorContains(t, apiErr, "Type=invalid_request")
	assert.ErrorContains(t, apiErr, "Params=[]")

}
