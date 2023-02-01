package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	goerrors "errors"

	"github.com/safedep/dry/api"
	"github.com/safedep/dry/utils"
)

type apiErrWrap struct {
	apiErr *api.ApiError
}

// AsApiError safely performs type conversion
// or build a generic Api Error object with message
func AsApiError(err error) (*apiErrWrap, bool) {
	if aErr, ok := err.(*apiErrWrap); ok {
		return aErr, true
	}

	error_type := api.ApiErrorTypeInternalError
	error_code := api.ApiErrorCodeAppGenericError
	error_msg := err.Error()

	return &apiErrWrap{
		apiErr: &api.ApiError{
			Type:    &error_type,
			Code:    &error_code,
			Message: &error_msg,
		},
	}, false
}

func BuildApiError(errType api.ApiErrorType, errCode api.ApiErrorCode,
	message string) *apiErrWrap {
	return &apiErrWrap{
		apiErr: &api.ApiError{
			Type:    &errType,
			Code:    &errCode,
			Message: &message,
		},
	}
}

func UnmarshalApiError(body []byte) (*apiErrWrap, bool) {
	apiErr := api.ApiError{}
	err := json.Unmarshal(body, &apiErr)
	if err != nil {
		return AsApiError(err)
	}

	if (apiErr.Type == nil) || (apiErr.Code == nil) {
		return AsApiError(goerrors.New("invalid API error model"))
	}

	return &apiErrWrap{
		apiErr: &apiErr,
	}, true
}

func (err *apiErrWrap) AddParam(key, val string) *apiErrWrap {
	if err.apiErr.Params == nil {
		err.apiErr.Params = &api.ApiError_Params{}
	}

	err.apiErr.Params.Set(key, struct {
		Key   *string `json:"key,omitempty"`
		Value *string `json:"value,omitempty"`
	}{
		Key:   &key,
		Value: &val,
	})

	return err
}

func (err *apiErrWrap) Error() string {
	params := "[]"
	if err.apiErr.Params != nil {
		params = "["
		for key, value := range err.apiErr.Params.AdditionalProperties {
			params = params + " "
			params = params + fmt.Sprintf("%s:\"%s\"", key, utils.SafelyGetValue(value.Value))
		}
		params = params + " ]"
	}

	return fmt.Sprintf("ApiError: Type=%s Code=%s Message=%s Params=%s",
		*err.apiErr.Type, *err.apiErr.Code, *err.apiErr.Message, params)
}

func (err *apiErrWrap) HttpCode() int {
	switch *err.apiErr.Code {
	case api.ApiErrorCodeAppPackageVersionNotFound:
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// Retriable returns true if the error indicates that the client should
// retry the request
func (err *apiErrWrap) Retriable() bool {
	return false
}

func (err *apiErrWrap) ApiError() *api.ApiError {
	return err.apiErr
}
