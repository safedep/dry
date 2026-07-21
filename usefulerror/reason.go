package usefulerror

import (
	"strings"

	errorv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/error/v1"
)

// ErrorReason is re-exported so callers can use the typed SafeDep business
// reason without importing the generated protobuf package directly.
type ErrorReason = errorv1.ErrorReason

// reasonWireOverrides maps a typed reason to a non-derivable ErrorInfo.reason
// string. Only reasons whose wire string predates the typed error model belong
// here: they are lowercase compatibility contracts that cannot be regenerated
// from the enum name. Every other reason derives its wire string from the enum
// (see ReasonWire), so this map must stay small.
var reasonWireOverrides = map[ErrorReason]string{
	errorv1.ErrorReason_ERROR_REASON_ENTITLEMENT_NOT_AVAILABLE: "entitlement_not_available",
	errorv1.ErrorReason_ERROR_REASON_QUOTA_EXCEEDED:            "quota_exceeded",
}

// wireReasonToEnum is the reverse of reasonWireOverrides, used to recover the
// typed reason from a legacy server that only emits google.rpc.ErrorInfo.
var wireReasonToEnum = func() map[string]ErrorReason {
	m := make(map[string]ErrorReason, len(reasonWireOverrides))
	for reason, wire := range reasonWireOverrides {
		m[wire] = reason
	}
	return m
}()

const reasonEnumPrefix = "ERROR_REASON_"

// ReasonWire returns the google.rpc.ErrorInfo.reason string for a typed reason.
//
// This is the single point that translates the enum into its wire string.
// Producers must never author the string by hand: keeping the mapping here is
// what stops the reason enum and the compatibility ErrorInfo from drifting
// apart. New reasons project to their UPPER_SNAKE_CASE name with the
// ERROR_REASON_ prefix stripped; the two grandfathered lowercase values are
// held in reasonWireOverrides.
func ReasonWire(reason ErrorReason) string {
	if wire, ok := reasonWireOverrides[reason]; ok {
		return wire
	}

	return strings.TrimPrefix(reason.String(), reasonEnumPrefix)
}

// reasonFromWire recovers the typed reason from an ErrorInfo.reason string.
// ok is false when the string matches no known reason, in which case the caller
// must fall back to the canonical gRPC code.
func reasonFromWire(wire string) (ErrorReason, bool) {
	if reason, ok := wireReasonToEnum[wire]; ok {
		return reason, true
	}

	if value, ok := errorv1.ErrorReason_value[reasonEnumPrefix+wire]; ok {
		return ErrorReason(value), true
	}

	return errorv1.ErrorReason_ERROR_REASON_UNSPECIFIED, false
}
