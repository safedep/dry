package usefulerror

import (
	"fmt"
	"testing"

	errorv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/error/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestReasonWire(t *testing.T) {
	tests := []struct {
		reason ErrorReason
		want   string
	}{
		{errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE, "entitlement_not_available"},
		{errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED, "quota_exceeded"},
		{errorv1.ErrorReason_ERROR_REASON_GITHUB_REAUTHORIZATION_REQUIRED, "GITHUB_REAUTHORIZATION_REQUIRED"},
		{errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE, "PROJECT_NOT_SCANNABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, ReasonWire(tt.reason))

			// Wire string must round-trip back to the same reason.
			got, ok := reasonFromWire(tt.want)
			assert.True(t, ok)
			assert.Equal(t, tt.reason, got)
		})
	}
}

func TestStatusBuilder_EmitsTypedAndLegacyDetails(t *testing.T) {
	err := NewStatus(codes.ResourceExhausted, "on-demand scan quota exhausted").
		WithReason(errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED).
		WithMetadata("tenantId", "t-123").
		WithDetail(&errdetails.QuotaFailure{
			Violations: []*errdetails.QuotaFailure_Violation{{Subject: "credits"}},
		}).
		Err()

	// Canonical code and message survive.
	code, ok := GRPCCode(err)
	require.True(t, ok)
	assert.Equal(t, codes.ResourceExhausted, code)

	// Typed reason is readable.
	reason, ok := ReasonOf(err)
	require.True(t, ok)
	assert.Equal(t, errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED, reason)

	// Derived ErrorInfo carries the compatibility projection, domain, metadata.
	info, ok := DetailAs[*errdetails.ErrorInfo](err)
	require.True(t, ok)
	assert.Equal(t, "quota_exceeded", info.GetReason())
	assert.Equal(t, DefaultErrorDomain, info.GetDomain())
	assert.Equal(t, "t-123", info.GetMetadata()["tenantId"])

	// Sibling standard detail is a peer, readable independently.
	quota, ok := DetailAs[*errdetails.QuotaFailure](err)
	require.True(t, ok)
	require.Len(t, quota.GetViolations(), 1)
	assert.Equal(t, "credits", quota.GetViolations()[0].GetSubject())
}

func TestStatusBuilder_WithDetailDedupesByType(t *testing.T) {
	err := NewStatus(codes.FailedPrecondition, "retry later").
		WithDetail(&errdetails.RetryInfo{}).
		WithDetail(&errdetails.RetryInfo{}).
		Err()

	st, _ := status.FromError(err)
	assert.Len(t, st.Details(), 1, "same detail type must appear at most once")
}

func TestStatusBuilder_ReasonOwnedDetailsWinOverCallerDetails(t *testing.T) {
	// A caller-supplied ErrorInfo must not override the one derived by WithReason.
	err := NewStatus(codes.PermissionDenied, "denied").
		WithReason(errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE).
		WithDetail(&errdetails.ErrorInfo{Reason: "attacker_controlled"}).
		Err()

	info, ok := DetailAs[*errdetails.ErrorInfo](err)
	require.True(t, ok)
	assert.Equal(t, "entitlement_not_available", info.GetReason())
}

func TestReasonOf_FromLegacyErrorInfo(t *testing.T) {
	// Older servers emit only ErrorInfo; ReasonOf must recover the typed reason.
	st, err := status.New(codes.PermissionDenied, "no access").
		WithDetails(&errdetails.ErrorInfo{Reason: "entitlement_not_available", Domain: DefaultErrorDomain})
	require.NoError(t, err)

	reason, ok := ReasonOf(st.Err())
	require.True(t, ok)
	assert.Equal(t, errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE, reason)
}

func TestReasonOf_UnknownIsFallback(t *testing.T) {
	// A gRPC error with no SafeDep reason must report ok=false so callers fall
	// back to the canonical code.
	_, ok := ReasonOf(status.Errorf(codes.NotFound, "missing"))
	assert.False(t, ok)

	// A non-gRPC error likewise.
	_, ok = ReasonOf(fmt.Errorf("plain error"))
	assert.False(t, ok)
}

func TestReasonOf_UnspecifiedTypedReasonIsFallback(t *testing.T) {
	// A typed ErrorDetail carrying UNSPECIFIED means "no business reason". It
	// must report ok=false so consumers fall back to the canonical code.
	st := status.New(codes.Internal, "boom")
	stProto := st.Proto()
	detail, err := anypb.New(errorv1.ErrorDetail_builder{
		Reason: errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED,
	}.Build())
	require.NoError(t, err)
	stProto.Details = append(stProto.Details, detail)

	_, ok := ReasonOf(status.FromProto(stProto).Err())
	assert.False(t, ok)
}

func TestReasonOf_UnknownNumericReasonIsFallback(t *testing.T) {
	// A reason value from a newer API that this build's enum does not define
	// must report ok=false rather than surface an unrecognized reason.
	unknownReason := errorv1.ErrorReason(999999)
	_, defined := errorv1.ErrorReason_name[int32(unknownReason)]
	require.False(t, defined, "test value must be absent from the generated enum")

	st := status.New(codes.Internal, "boom")
	stProto := st.Proto()
	detail, err := anypb.New(errorv1.ErrorDetail_builder{Reason: unknownReason}.Build())
	require.NoError(t, err)
	stProto.Details = append(stProto.Details, detail)

	_, ok := ReasonOf(status.FromProto(stProto).Err())
	assert.False(t, ok)
}

func TestDetailAs_UnwrapsNestedAny(t *testing.T) {
	// Mirror the control-tower double-wrap: an ErrorDetail nested two Any layers
	// deep must still be extractable.
	packed, err := anypb.New(errorv1.ErrorDetail_builder{
		Reason: errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE,
	}.Build())
	require.NoError(t, err)
	packed, err = anypb.New(packed)
	require.NoError(t, err)

	st := status.New(codes.InvalidArgument, "bad project")
	stProto := st.Proto()
	stProto.Details = append(stProto.Details, packed)

	detail, ok := DetailAs[*errorv1.ErrorDetail](status.FromProto(stProto).Err())
	require.True(t, ok)
	assert.Equal(t, errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE, detail.GetReason())
}

func TestAsUsefulError_TypedReasonDrivesPresentation(t *testing.T) {
	err := NewStatus(codes.PermissionDenied, "subscription required").
		WithReason(errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE).
		Err()

	useful, ok := AsUsefulError(err)
	require.True(t, ok)
	assert.Equal(t, ErrMissingEntitlements, useful.Code())
	assert.Equal(t, "Feature unavailable", useful.HumanError())
	assert.Contains(t, useful.Help(), "SafeDep subscription")
	// AdditionalHelp falls back to the status message when not set explicitly.
	assert.Contains(t, useful.AdditionalHelp(), "subscription required")
}

func TestAsUsefulError_TypedReasonBeatsGenericCodeConverter(t *testing.T) {
	// PROJECT_NOT_SCANNABLE rides on InvalidArgument. Without the typed reason
	// the generic converter would classify it as a bad request; the typed
	// presentation must win.
	err := NewStatus(codes.InvalidArgument, "no scannable source").
		WithReason(errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE).
		Err()

	useful, ok := AsUsefulError(err)
	require.True(t, ok)
	assert.Equal(t, ErrBadRequest, useful.Code())
	assert.Equal(t, "Project not scannable", useful.HumanError())
	assert.Equal(t,
		"Grant the SafeDep GitHub App access to this repository, wait for project sync, then retry.",
		useful.Help())
	assert.Equal(t,
		"https://docs.safedep.io/governance/integrations/github",
		useful.ReferenceURL())
}

func TestAsUsefulError_UnregisteredReasonFallsBackToCode(t *testing.T) {
	// A typed reason with no registered presentation must defer to the generic
	// per-code converter rather than producing a blank UsefulError.
	err := NewStatus(codes.NotFound, "missing").
		WithReason(errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED).
		Err()

	useful, ok := AsUsefulError(err)
	require.True(t, ok)
	assert.Equal(t, ErrNotFound, useful.Code())
}
