package usefulerror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConvertGRPCToUsefulError(t *testing.T) {
	tests := []struct {
		name              string
		input             error
		expectedCode      string
		expectedHuman     string
		expectConversion  bool
		additionalHelpSub string // substring expected to be present in AdditionalHelp (optional)
	}{
		{
			name:             "unauthenticated -> authentication failed",
			input:            status.Errorf(codes.Unauthenticated, "auth failed"),
			expectedCode:     ErrAuthenticationFailed,
			expectedHuman:    "Authentication failed",
			expectConversion: true,
		},
		{
			name:              "permission denied -> authorization failed",
			input:             status.Errorf(codes.PermissionDenied, "no access"),
			expectedCode:      ErrAuthorizationFailed,
			expectedHuman:     "Permission denied",
			expectConversion:  true,
			additionalHelpSub: "no access",
		},
		{
			name:              "invalid argument -> bad request",
			input:             status.Errorf(codes.InvalidArgument, "bad field"),
			expectedCode:      ErrBadRequest,
			expectedHuman:     "Invalid request",
			expectConversion:  true,
			additionalHelpSub: "bad field",
		},
		{
			name:              "not found -> resource not found",
			input:             status.Errorf(codes.NotFound, "missing"),
			expectedCode:      ErrNotFound,
			expectedHuman:     "Resource not found",
			expectConversion:  true,
			additionalHelpSub: "missing",
		},
		{
			name:              "already exists -> conflict",
			input:             status.Errorf(codes.AlreadyExists, "exists"),
			expectedCode:      ErrConflict,
			expectedHuman:     "Resource already exists",
			expectConversion:  true,
			additionalHelpSub: "exists",
		},
		{
			name:              "resource exhausted -> quota exceeded",
			input:             status.Errorf(codes.ResourceExhausted, "quota exceeded"),
			expectedCode:      ErrQuotaExceeded,
			expectedHuman:     "Quota exceeded",
			expectConversion:  true,
			additionalHelpSub: "quota exceeded",
		},
		{
			name:              "deadline exceeded -> request timed out",
			input:             status.Errorf(codes.DeadlineExceeded, "timed out"),
			expectedCode:      ErrGatewayTimeout,
			expectedHuman:     "Request timed out",
			expectConversion:  true,
			additionalHelpSub: "timed out",
		},
		{
			name:              "unavailable -> service unavailable",
			input:             status.Errorf(codes.Unavailable, "down"),
			expectedCode:      ErrServiceUnavailable,
			expectedHuman:     "Service unavailable",
			expectConversion:  true,
			additionalHelpSub: "down",
		},
		{
			name:              "internal -> internal server error",
			input:             status.Errorf(codes.Internal, "panic"),
			expectedCode:      ErrInternalServerError,
			expectedHuman:     "Internal server error",
			expectConversion:  true,
			additionalHelpSub: "panic",
		},
		{
			name:              "unimplemented -> feature not implemented",
			input:             status.Errorf(codes.Unimplemented, "not implemented"),
			expectedCode:      ErrInternalServerError,
			expectedHuman:     "Feature not implemented",
			expectConversion:  true,
			additionalHelpSub: "not implemented",
		},
		{
			name:              "canceled -> request cancelled",
			input:             status.Errorf(codes.Canceled, "client cancelled"),
			expectedCode:      ErrNetworkError,
			expectedHuman:     "Request cancelled",
			expectConversion:  true,
			additionalHelpSub: "client cancelled",
		},
		{
			name:             "non-grpc error should not convert",
			input:            fmt.Errorf("some other error"),
			expectConversion: false,
		},
		{
			name:             "nil error should not convert",
			input:            nil,
			expectConversion: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := AsUsefulError(tt.input)
			if !tt.expectConversion {
				assert.False(t, ok)
				assert.Nil(t, result)
				return
			}

			assert.True(t, ok)
			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedCode, result.Code(), "unexpected code")
			assert.Equal(t, tt.expectedHuman, result.HumanError(), "unexpected human error")

			if tt.additionalHelpSub != "" {
				// AdditionalHelp may be the gRPC status message; ensure substring present.
				assert.Contains(t, result.AdditionalHelp(), tt.additionalHelpSub)
			}
		})
	}
}

func TestConvertGRPCToUsefulError_NestedWrapped(t *testing.T) {
	// Ensure conversion works even when the gRPC error is wrapped inside other errors
	inner := status.Errorf(codes.PermissionDenied, "missing entitlements")
	wrapped := fmt.Errorf("handler error: %w", inner)

	result, ok := AsUsefulError(wrapped)
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, ErrAuthorizationFailed, result.Code())
	assert.Equal(t, "Permission denied", result.HumanError())
	assert.Contains(t, result.AdditionalHelp(), "missing entitlements")
}
