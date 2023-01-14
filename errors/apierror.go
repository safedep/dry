package errors

import (
	"fmt"
	"net/http"
)

type apiErrWrap struct {
	apiErr *ApiError
}

func AsApiError(err error) (*apiErrWrap, bool) {
	if aErr, ok := err.(*apiErrWrap); ok {
		return aErr, true
	}

	return &apiErrWrap{}, false
}

func BuildApiError(code, message, errType string) *apiErrWrap {
	return &apiErrWrap{
		apiErr: &ApiError{
			Code:    &code,
			Message: &message,
			Type:    &errType,
		},
	}
}

func (err *apiErrWrap) AddParam(key, val string) *apiErrWrap {
	if err.apiErr.Params == nil {
		err.apiErr.Params = &ApiError_Params{}
	}

	err.apiErr.Params.AdditionalProperties[key] = struct {
		Key   *string `json:"key,omitempty"`
		Value *string `json:"value,omitempty"`
	}{
		&key,
		&val,
	}

	return err
}

func (err *apiErrWrap) AsError() error {
	return err
}

func (err *apiErrWrap) Error() string {
	return fmt.Sprintf("ApiError: Code=%s Message=%s",
		*err.apiErr.Code, *err.apiErr.Message)
}

func (err *apiErrWrap) HttpCode() int {
	switch *err.apiErr.Code {
	case "404":
		return http.StatusNotFound
	case "401":
		return http.StatusUnauthorized
	case "403":
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// Retriable returns true if the error indicates that the client should
// retry the request
func (err *apiErrWrap) Retriable() bool {
	return false
}

func (err *apiErrWrap) ApiError() *ApiError {
	return err.apiErr
}
