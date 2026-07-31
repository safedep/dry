package usefulerror

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestConvertGRPCToUsefulError(t *testing.T) {
	tests := []struct {
		name              string
		input             error
		expectedCode      string
		expectedHuman     string
		expectedHelp      string
		expectedReference string
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
			expectedHelp:      "Retry the operation. If the error continues, contact SafeDep support.",
			expectedReference: "https://docs.safedep.io/community",
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
			if tt.expectedHelp != "" {
				assert.Equal(t, tt.expectedHelp, result.Help(), "unexpected help")
			}
			if tt.expectedReference != "" {
				assert.Equal(t, tt.expectedReference, result.ReferenceURL(), "unexpected reference URL")
			}

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

func TestConvertGRPCToUsefulError_PermissionDenied_WithDetails(t *testing.T) {
	// Build a gRPC status with ErrorInfo reason=entitlement_not_available
	st := status.New(codes.PermissionDenied, "no access")
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: ErrAppEntitlementNotAvailable,
		Domain: "safedep.io",
		Metadata: map[string]string{
			"feature": "some_feature",
		},
	})
	assert.NoError(t, err)

	result, ok := AsUsefulError(withDetails.Err())
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, ErrMissingEntitlements, result.Code())
	assert.Equal(t, "Permission denied", result.HumanError())
	assert.Equal(t, "Access to this feature requires a SafeDep subscription. See https://safedep.io/pricing", result.Help())
	assert.Contains(t, result.AdditionalHelp(), "no access")
}

func TestConvertGRPCToUsefulError_ResourceExhausted_WithDetails_FeatureNotAvailable(t *testing.T) {
	// Build a gRPC status with ErrorInfo reason=quota_exceeded and metadata reason=feature_not_available
	st := status.New(codes.ResourceExhausted, "quota exceeded")
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: ErrAppQuotaExceeded,
		Domain: "safedep.io",
		Metadata: map[string]string{
			"reason":  ErrAppQuotaReasonFeatureNotAvailable,
			"feature": "advanced_feature",
			"tier":    "basic",
		},
	})
	assert.NoError(t, err)

	result, ok := AsUsefulError(withDetails.Err())
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, ErrMissingEntitlements, result.Code())
	assert.Equal(t, "Feature unavailable", result.HumanError())
	assert.Equal(t, "Enable this feature for your subscription or upgrade your plan, then retry.", result.Help())
	assert.Equal(t, "https://safedep.io/pricing", result.ReferenceURL())
	assert.Contains(t, result.AdditionalHelp(), "quota exceeded")
}

func TestConvertGRPCToUsefulError_PermissionDenied_WithAnypbDetail(t *testing.T) {
	// Test the *anypb.Any fallback path in getErrorInfoFromGrpcStatusDetails.
	ei := &errdetails.ErrorInfo{
		Reason: ErrAppEntitlementNotAvailable,
		Domain: "safedep.io",
		Metadata: map[string]string{
			"feature": "some_feature",
		},
	}

	eiBytes, err := proto.Marshal(ei)
	assert.NoError(t, err)

	st := status.New(codes.PermissionDenied, "no access")
	stProto := st.Proto()
	stProto.Details = append(stProto.Details, &anypb.Any{
		TypeUrl: "type.googleapis.com/google.rpc.ErrorInfo",
		Value:   eiBytes,
	})

	reconstructed := status.FromProto(stProto)
	result, ok := AsUsefulError(reconstructed.Err())

	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, ErrMissingEntitlements, result.Code())
	assert.Equal(t, "Permission denied", result.HumanError())
	assert.Equal(t, "Access to this feature requires a SafeDep subscription. See https://safedep.io/pricing", result.Help())
	assert.Contains(t, result.AdditionalHelp(), "no access")
}

func TestConvertGRPCToUsefulError_PermissionDenied_WithNestedAnyDetail(t *testing.T) {
	// Regression: control-tower's serror.Add used to re-pack already-packed
	// Any details on every wrap. The entitlement error passed through two
	// Add calls (service executor + API handler), so the ErrorInfo arrived
	// nested two Any layers deep. Extraction must unwrap nested Any layers.
	ei := &errdetails.ErrorInfo{
		Reason: ErrAppEntitlementNotAvailable,
		Domain: "safedep.io",
		Metadata: map[string]string{
			"feature": "some_feature",
		},
	}

	packed, err := anypb.New(ei)
	assert.NoError(t, err)

	for depth := 1; depth <= 2; depth++ {
		packed, err = anypb.New(packed)
		assert.NoError(t, err)

		st := status.New(codes.PermissionDenied, "no access")
		stProto := st.Proto()
		stProto.Details = append(stProto.Details, packed)

		result, ok := AsUsefulError(status.FromProto(stProto).Err())
		assert.True(t, ok)
		assert.NotNil(t, result)
		assert.Equalf(t, ErrMissingEntitlements, result.Code(), "extra Any wrap layers: %d (total nesting %d)", depth, depth+1)
		assert.Equal(t, "Access to this feature requires a SafeDep subscription. See https://safedep.io/pricing", result.Help())
	}
}

func TestErrorInfoFromDetailNestingBound(t *testing.T) {
	var msg proto.Message = &errdetails.ErrorInfo{Reason: ErrAppEntitlementNotAvailable}
	for range maxErrorInfoAnyNesting {
		packed, err := anypb.New(msg)
		assert.NoError(t, err)
		msg = packed
	}

	ei, ok := errorInfoFromDetail(msg)
	assert.True(t, ok, "ErrorInfo at exactly maxErrorInfoAnyNesting layers must be found")
	assert.Equal(t, ErrAppEntitlementNotAvailable, ei.GetReason())

	over, err := anypb.New(msg)
	assert.NoError(t, err)
	_, ok = errorInfoFromDetail(over)
	assert.False(t, ok, "ErrorInfo beyond maxErrorInfoAnyNesting layers must be rejected")
}

func TestConvertGRPCToUsefulError_UnrelatedNestedDetailIsNotUnwrapped(t *testing.T) {
	// A nested Any wrapping something other than ErrorInfo (or another Any)
	// must not be decoded or classified as an entitlement failure.
	retry, err := anypb.New(&errdetails.RetryInfo{})
	assert.NoError(t, err)

	nested, err := anypb.New(retry)
	assert.NoError(t, err)

	st := status.New(codes.PermissionDenied, "no access")
	stProto := st.Proto()
	stProto.Details = append(stProto.Details, nested)

	result, ok := AsUsefulError(status.FromProto(stProto).Err())
	assert.True(t, ok)
	assert.NotNil(t, result)
	assert.Equal(t, ErrAuthorizationFailed, result.Code())
}

func TestConvertGRPCToUsefulError_ResourceExhausted_WithDetails_LimitReached(t *testing.T) {
	// Build a gRPC status with ErrorInfo reason=quota_exceeded and metadata reason=limit_reached
	st := status.New(codes.ResourceExhausted, "rate limit reached")
	withDetails, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: ErrAppQuotaExceeded,
		Domain: "safedep.io",
		Metadata: map[string]string{
			"reason": ErrAppQuotaReasonLimitReached,
		},
	})
	assert.NoError(t, err)

	result, ok := AsUsefulError(withDetails.Err())
	assert.True(t, ok)
	assert.NotNil(t, result)
	// For limit_reached, we map to rate limit exceeded with tailored help
	assert.Equal(t, ErrRateLimitExceeded, result.Code())
	assert.Equal(t, "Quota exceeded", result.HumanError())
	assert.Equal(t, "Feature quota limit exceeded. Upgrade your plan for higher limit", result.Help())
	assert.Contains(t, result.AdditionalHelp(), "rate limit reached")
}
