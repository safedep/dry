package project

import (
	"fmt"

	specv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/policy/spec/v1"
)

// checkHeader applies the header check. A failure rejects the whole
// document, so the tool keeps the policy that it applies now.
func checkHeader(policy *specv1.ResolvedPolicy, expect Expect) (Header, error) {
	if !policy.HasHeader() {
		return Header{}, fmt.Errorf("%w: no header", ErrHeader)
	}

	h := policy.GetHeader()
	if expect.Tenant != "" && h.GetTenant() != expect.Tenant {
		return Header{}, fmt.Errorf("%w: tenant %q is not %q", ErrHeader, h.GetTenant(), expect.Tenant)
	}

	kind := h.GetSubject().GetKind()
	if kind != expect.SubjectKind || kind == specv1.SubjectKind_SUBJECT_KIND_UNSPECIFIED {
		return Header{}, fmt.Errorf("%w: subject kind %s is not %s", ErrHeader, kind, expect.SubjectKind)
	}

	switch kind {
	case specv1.SubjectKind_SUBJECT_KIND_ENDPOINT:
		if expect.MachineID == "" || h.GetMachineId() != expect.MachineID {
			return Header{}, fmt.Errorf("%w: the machine id is not the machine id of this endpoint", ErrHeader)
		}

	case specv1.SubjectKind_SUBJECT_KIND_PROJECT:
		if expect.ProjectID == "" || h.GetSubject().GetId() != expect.ProjectID {
			return Header{}, fmt.Errorf("%w: subject %q is not project %q", ErrHeader, h.GetSubject().GetId(), expect.ProjectID)
		}

	default:
		return Header{}, fmt.Errorf("%w: unknown subject kind %d", ErrHeader, int32(kind))
	}

	header := Header{
		Tenant:                h.GetTenant(),
		SubjectKind:           kind,
		SubjectID:             h.GetSubject().GetId(),
		MachineID:             h.GetMachineId(),
		Revision:              h.GetRevision(),
		DefaultPolicyRevision: h.GetDefaultPolicyRevision(),
	}

	if h.HasIssuedAt() {
		header.IssuedAt = h.GetIssuedAt().AsTime()
	}

	if h.HasNextExpiryAt() {
		header.NextExpiryAt = h.GetNextExpiryAt().AsTime()
	}

	for _, o := range h.GetMatchedOverrides() {
		header.MatchedOverrides = append(header.MatchedOverrides, MatchedOverride{
			ID:       o.GetOverrideId(),
			Name:     o.GetName(),
			Priority: o.GetPriority(),
		})
	}

	return header, nil
}
