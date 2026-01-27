package usefulerror

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() {
	// Unauthenticated -> authentication failure
	registerInternalErrorConverter("grpc/unauthenticated", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unauthenticated {
			return NewUsefulError().
				WithCode(ErrAuthenticationFailed).
				WithHumanError("Authentication failed").
				WithHelp("Check your credentials and try again.").
				WithAdditionalHelp("If you are using a token, make sure it is valid and not expired.").
				Wrap(err), true
		}
		return nil, false
	})

	// PermissionDenied -> authorization failure
	registerInternalErrorConverter("grpc/permission_denied", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.PermissionDenied {
			return NewUsefulError().
				WithCode(ErrAuthorizationFailed).
				WithHumanError("Permission denied").
				WithHelp("You do not have permission to perform this action.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// InvalidArgument -> bad request
	registerInternalErrorConverter("grpc/invalid_argument", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument {
			return NewUsefulError().
				WithCode(ErrBadRequest).
				WithHumanError("Invalid request").
				WithHelp("Verify the request parameters and types and try again.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// NotFound -> resource not found
	registerInternalErrorConverter("grpc/not_found", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return NewUsefulError().
				WithCode(ErrNotFound).
				WithHumanError("Resource not found").
				WithHelp("Ensure the resource identifier is correct and the resource exists.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// AlreadyExists -> conflict
	registerInternalErrorConverter("grpc/already_exists", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			return NewUsefulError().
				WithCode(ErrConflict).
				WithHumanError("Resource already exists").
				WithHelp("Check for duplicate resources or identifiers before retrying.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// ResourceExhausted -> rate limiting / quota exceeded
	registerInternalErrorConverter("grpc/resource_exhausted", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.ResourceExhausted {
			return NewUsefulError().
				WithCode(ErrRateLimitExceeded).
				WithHumanError("Rate limit exceeded").
				WithHelp("Reduce request frequency or increase your quota.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// DeadlineExceeded -> gateway / timeout
	registerInternalErrorConverter("grpc/deadline_exceeded", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.DeadlineExceeded {
			return NewUsefulError().
				WithCode(ErrGatewayTimeout).
				WithHumanError("Request timed out").
				WithHelp("The operation took too long to complete. Try again later or increase the timeout.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// Unavailable -> temporary service outage / network issue
	registerInternalErrorConverter("grpc/unavailable", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return NewUsefulError().
				WithCode(ErrServiceUnavailable).
				WithHumanError("Service unavailable").
				WithHelp("The service is temporarily unavailable. Retry after a short delay.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// Internal -> internal server error
	registerInternalErrorConverter("grpc/internal", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Internal {
			return NewUsefulError().
				WithCode(ErrInternalServerError).
				WithHumanError("Internal server error").
				WithHelp("An internal error occurred while processing the request.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// Unimplemented -> feature not implemented
	registerInternalErrorConverter("grpc/unimplemented", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
			return NewUsefulError().
				WithCode(ErrInternalServerError).
				WithHumanError("Feature not implemented").
				WithHelp("This operation or endpoint is not implemented.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// Canceled -> client cancelled or network hiccup; map to network error
	registerInternalErrorConverter("grpc/canceled", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
			return NewUsefulError().
				WithCode(ErrNetworkError).
				WithHumanError("Request cancelled").
				WithHelp("The request was cancelled before completion.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})

	// Unknown -> generic internal
	registerInternalErrorConverter("grpc/unknown", func(err error) (UsefulError, bool) {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unknown {
			return NewUsefulError().
				WithCode(ErrInternalServerError).
				WithHumanError("Unknown error").
				WithHelp("An unknown error occurred. Check logs for more details.").
				WithAdditionalHelp(st.Message()).
				Wrap(err), true
		}
		return nil, false
	})
}
