package errors

import (
	"fmt"
	"net/http"

	"github.com/safedep/dry/api"
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
	return fmt.Sprintf("ApiError: Code=%s Message=%s",
		*err.apiErr.Code, *err.apiErr.Message)
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
