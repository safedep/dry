package usefulerror

import (
	"sync"

	errorv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/error/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ReasonOf extracts the typed SafeDep business reason from an error, looking
// through wrapping. It prefers the typed safedep.messages.error.v1.ErrorDetail
// and falls back to recovering the reason from a legacy google.rpc.ErrorInfo,
// so it works against both current and older servers.
//
// ok is false when the error carries no recognizable SafeDep reason (it is not
// a gRPC status, has no reason detail, or the reason is UNSPECIFIED or an
// unknown numeric value). Callers must then fall back to the canonical gRPC
// code, which GRPCCode returns.
func ReasonOf(err error) (ErrorReason, bool) {
	if reason, ok := typedReasonOf(err); ok {
		return reason, true
	}

	st, ok := grpcStatusOf(err)
	if !ok {
		return errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED, false
	}

	if info, ok := getErrorInfoFromGrpcStatusDetails(st); ok {
		return reasonFromWire(info.GetReason())
	}

	return errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED, false
}

// GRPCCode returns the canonical gRPC status code of an error, looking through
// wrapping. It is the fallback every consumer should rely on when ReasonOf or a
// specific detail is absent. ok is false when err is nil or is not a gRPC
// status error.
func GRPCCode(err error) (codes.Code, bool) {
	if err == nil {
		return codes.OK, false
	}

	st, ok := status.FromError(err)
	if !ok || st == nil {
		return codes.Unknown, false
	}

	return st.Code(), true
}

// DetailAs extracts the first status detail of type T from an error, looking
// through wrapping. It is the typed way to read a rich detail such as a
// standard google.rpc.* message or a SafeDep-specific detail, and is the seam
// for details added to the error model in future:
//
//	if q, ok := usefulerror.DetailAs[*scanv1.OnDemandScanQuotaDetail](err); ok {
//		// use q to drive a specific recovery
//	}
//
// ok is false when the error is not a gRPC status or carries no detail of type
// T. Consumers must ignore details they do not recognize and fall back to the
// canonical code.
func DetailAs[T proto.Message](err error) (T, bool) {
	var zero T

	st, ok := grpcStatusOf(err)
	if !ok {
		return zero, false
	}

	for _, detail := range st.Details() {
		msg, ok := detail.(proto.Message)
		if !ok {
			continue
		}

		if v, ok := detailAsType[T](msg); ok {
			return v, true
		}
	}

	return zero, false
}

// detailAsType asserts msg to T, unwrapping nested google.protobuf.Any layers.
// Nested layers occur when an upstream server re-packs an already-packed detail
// while propagating an error; the same bound as ErrorInfo extraction guards
// against hostile deep nesting.
func detailAsType[T proto.Message](msg proto.Message) (T, bool) {
	var zero T

	for range maxErrorInfoAnyNesting + 1 {
		if v, ok := msg.(T); ok {
			return v, true
		}

		packed, ok := msg.(*anypb.Any)
		if !ok {
			return zero, false
		}

		unpacked, err := packed.UnmarshalNew()
		if err != nil {
			return zero, false
		}
		msg = unpacked
	}

	return zero, false
}

// typedReasonOf reads the reason strictly from the typed ErrorDetail. It never
// falls back to ErrorInfo so that the UsefulError conversion path stays
// behavior-compatible with legacy statuses that carry only an ErrorInfo.
//
// ok is false unless the reason is actionable: it must be neither UNSPECIFIED
// nor a numeric value absent from this build's generated enum (which happens
// when a newer server sends a reason this client does not yet know). Callers
// then fall back to the canonical code.
func typedReasonOf(err error) (ErrorReason, bool) {
	detail, ok := DetailAs[*errorv1.ErrorDetail](err)
	if !ok {
		return errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED, false
	}

	reason := detail.GetReason()
	if !isActionableReason(reason) {
		return errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED, false
	}

	return reason, true
}

// isActionableReason reports whether a reason is safe to act on: it is defined
// (present in the generated enum) and is not the UNSPECIFIED sentinel.
func isActionableReason(reason ErrorReason) bool {
	if reason == errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED {
		return false
	}

	_, known := errorv1.ErrorReason_name[int32(reason)]
	return known
}

func grpcStatusOf(err error) (*status.Status, bool) {
	if err == nil {
		return nil, false
	}

	st, ok := status.FromError(err)
	if !ok || st == nil {
		return nil, false
	}

	return st, true
}

// ReasonPresentation is the user-facing rendering of a business reason: it maps
// a typed ErrorReason to the fields of a UsefulError. AdditionalHelp is
// optional; when empty, the gRPC status message is used so that server-provided
// context is not lost.
type ReasonPresentation struct {
	Code           string
	HumanError     string
	Help           string
	AdditionalHelp string
	ReferenceURL   string
}

var (
	reasonPresentationMu       sync.RWMutex
	reasonPresentationRegistry = make(map[ErrorReason]ReasonPresentation)
)

// RegisterReason registers or overrides the presentation for a business reason.
// Applications call it to teach usefulerror how to render reasons specific to
// them, or to override a default. It is safe for concurrent use.
func RegisterReason(reason ErrorReason, presentation ReasonPresentation) {
	reasonPresentationMu.Lock()
	defer reasonPresentationMu.Unlock()

	reasonPresentationRegistry[reason] = presentation
}

func lookupReasonPresentation(reason ErrorReason) (ReasonPresentation, bool) {
	reasonPresentationMu.RLock()
	defer reasonPresentationMu.RUnlock()

	p, ok := reasonPresentationRegistry[reason]
	return p, ok
}

// convertFromTypedReason turns an error carrying a typed ErrorDetail into a
// UsefulError using the registered presentation for its reason. It is wired
// ahead of the generic per-code converters so a typed reason wins over the
// broad code classification, and returns ok=false (deferring to those
// converters) when no typed reason or presentation is available.
func convertFromTypedReason(err error) (UsefulError, bool) {
	reason, ok := typedReasonOf(err)
	if !ok {
		return nil, false
	}

	presentation, ok := lookupReasonPresentation(reason)
	if !ok {
		return nil, false
	}

	additionalHelp := presentation.AdditionalHelp
	if additionalHelp == "" {
		if st, ok := grpcStatusOf(err); ok {
			additionalHelp = st.Message()
		}
	}

	return NewUsefulError().
		WithCode(presentation.Code).
		WithHumanError(presentation.HumanError).
		WithHelp(presentation.Help).
		WithAdditionalHelp(additionalHelp).
		WithReferenceURL(presentation.ReferenceURL).
		Wrap(err), true
}

// Default presentations for the reasons published in the initial error model.
// Applications may override any of these via RegisterReason.
func init() {
	RegisterReason(errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE, ReasonPresentation{
		Code:       ErrMissingEntitlements,
		HumanError: "Feature unavailable",
		Help:       "Access to this feature requires a SafeDep subscription. See https://safedep.io/pricing",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED, ReasonPresentation{
		Code:       ErrQuotaExceeded,
		HumanError: "Quota exceeded",
		Help:       "Reduce request frequency or upgrade your plan for a higher limit.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_GITHUB_INSTALLATION_LINK_CONFLICT, ReasonPresentation{
		Code:       ErrConflict,
		HumanError: "GitHub installation already linked",
		Help:       "This GitHub installation is linked to another tenant. Unlink it there before retrying.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_GITHUB_INSTALLATION_NOT_ACCESSIBLE, ReasonPresentation{
		Code:       ErrAuthorizationFailed,
		HumanError: "GitHub installation not accessible",
		Help:       "Confirm the GitHub installation belongs to your account and try again.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_GITHUB_INSTALLATION_SUSPENDED, ReasonPresentation{
		Code:       ErrAuthorizationFailed,
		HumanError: "GitHub installation suspended",
		Help:       "GitHub suspended this installation. Restore it from your GitHub settings and retry.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_GITHUB_REAUTHORIZATION_REQUIRED, ReasonPresentation{
		Code:       ErrAuthenticationFailed,
		HumanError: "GitHub authorization required",
		Help:       "Re-authorize the SafeDep GitHub app to continue.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_GITHUB_REPOSITORY_NOT_ACCESSIBLE, ReasonPresentation{
		Code:       ErrNotFound,
		HumanError: "GitHub repository not accessible",
		Help:       "Grant the SafeDep GitHub app access to this repository and retry.",
	})

	RegisterReason(errorv1.ErrorReason_ERROR_REASON_PROJECT_NOT_SCANNABLE, ReasonPresentation{
		Code:         ErrBadRequest,
		HumanError:   "Project not scannable",
		Help:         "Grant the SafeDep GitHub App access to this repository, wait for project sync, then retry.",
		ReferenceURL: "https://docs.safedep.io/governance/integrations/github",
	})
}
