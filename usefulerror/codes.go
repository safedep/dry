package usefulerror

// Error codes for standard errors related to the application.
// The internal converters should use one of the standard error codes.
const (
	ErrMissingEntitlements  = "missing_entitlements"
	ErrAuthenticationFailed = "authentication_failed"
	ErrAuthorizationFailed  = "authorization_failed"
	ErrRateLimitExceeded    = "rate_limit_exceeded"
	ErrInternalServerError  = "internal_server_error"
	ErrBadRequest           = "bad_request"
	ErrNotFound             = "not_found"
	ErrConflict             = "conflict"
	ErrServiceUnavailable   = "service_unavailable"
	ErrGatewayTimeout       = "gateway_timeout"
	ErrNetworkError         = "network_error"
)
