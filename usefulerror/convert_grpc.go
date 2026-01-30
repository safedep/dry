package usefulerror

import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ErrAppEntitlementNotAvailable        = "entitlement_not_available"
	ErrAppQuotaExceeded                  = "quota_exceeded"
	ErrAppQuotaReasonFeatureNotAvailable = "feature_not_available"
	ErrAppQuotaReasonLimitReached        = "limit_reached"
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
			help := "You do not have permission to perform this action."
			code := ErrAuthorizationFailed

			if errInfo, ok := getErrorInfoFromGrpcStatusDetails(st); ok {
				switch errInfo.Reason {
				case ErrAppEntitlementNotAvailable:
					code = ErrMissingEntitlements
					help = "Access to this feature requires a SafeDep subscription. See https://safedep.io/pricing"
				}
			}

			return NewUsefulError().
				WithCode(code).
				WithHumanError("Permission denied").
				WithHelp(help).
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
			help := "Reduce request frequency or increase your quota."
			code := ErrQuotaExceeded
			humanError := "Quota exceeded"

			if errInfo, ok := getErrorInfoFromGrpcStatusDetails(st); ok {
				switch errInfo.Reason {
				case ErrAppQuotaExceeded:
					reason := errInfo.Metadata["reason"]
					switch reason {
					case ErrAppQuotaReasonLimitReached:
						help = "Feature quota limit exceeded. Upgrade your plan for higher limit"
						code = ErrRateLimitExceeded
					case ErrAppQuotaReasonFeatureNotAvailable:
						help = "Feature not available for your subscription tier."
						code = ErrMissingEntitlements
						humanError = "Feature unavailable"
					}
				}
			}

			return NewUsefulError().
				WithCode(code).
				WithHumanError(humanError).
				WithHelp(help).
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
}

func getErrorInfoFromGrpcStatusDetails(st *status.Status) (*errdetails.ErrorInfo, bool) {
	for _, d := range st.Details() {
		switch det := d.(type) {
		case *errdetails.ErrorInfo:
			return det, true
		}
	}

	return nil, false
}
