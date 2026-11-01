package configsync

import (
	"context"
	"fmt"
	"time"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	policyv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/policy/v1"
	"github.com/safedep/dry/log"
)

// FetchRequest is one fetch of the resolved policy of an endpoint.
type FetchRequest struct {
	// TenantDomain is the tenant of the credentials of the data-plane
	// client. A tenant that differs from the tenant of the cache resets
	// highest_revision and sends an empty digest.
	TenantDomain string

	// Endpoint is the identity that the check-in of the tool sends, so the
	// fetch and the check-in name one endpoint.
	Endpoint *controltowerv1.EndpointIdentity

	// Labels are the "managed/" and "reported/" labels of the endpoint, as
	// endpointsync.ReportedLabels gives them.
	Labels []*controltowerv1.Label

	// ToolName and ToolVersion name the tool that fetches.
	ToolName    string
	ToolVersion string

	// Validate checks the bytes of a new policy before the commit. A tool
	// passes its header check and its projection. An error rejects the
	// policy and keeps the cache. It is required, because the fetcher never
	// decodes the bytes.
	Validate func(raw []byte) error

	// CapabilitySources is optional. It runs after Validate passes, on the
	// same bytes, and returns the capability_sources of the new commit: the
	// digest of the policy file that each failed capability comes from. The
	// store then keeps those files. A tool reads the current state with
	// Store.State to compute it. The fetcher drops a source on a change of
	// tenant, a source that names the new policy, and a digest that is not
	// valid.
	CapabilitySources func(raw []byte) map[string]string
}

// FetchStatus tells what a successful fetch did.
type FetchStatus int

const (
	// FetchNotModified means that the cache holds the policy of the server.
	// Only fetched_at moved.
	FetchNotModified FetchStatus = iota + 1

	// FetchModified means that the fetcher committed a new policy.
	FetchModified
)

func (s FetchStatus) String() string {
	switch s {
	case FetchNotModified:
		return "not_modified"
	case FetchModified:
		return "modified"
	default:
		return "unknown"
	}
}

// FetchResult is the result of a successful fetch.
type FetchResult struct {
	Status FetchStatus

	// Digest and Revision name the current policy of the cache after the
	// fetch.
	Digest   string
	Revision uint64
}

// Fetcher fetches the resolved policy and commits it to a Store.
type Fetcher struct {
	store     *Store
	transport Transport
	now       func() time.Time
}

// NewFetcher returns a fetcher that writes to store through transport.
func NewFetcher(store *Store, transport Transport) (*Fetcher, error) {
	if store == nil {
		return nil, fmt.Errorf("%w: store is required", ErrInvalidRequest)
	}
	if transport == nil {
		return nil, fmt.Errorf("%w: transport is required", ErrInvalidRequest)
	}
	return &Fetcher{store: store, transport: transport, now: time.Now}, nil
}

// Fetch asks the server for the resolved policy and commits it when every
// check passes. A failed fetch keeps the cache, records last_attempt_at and
// last_error, and returns the error. A gRPC error is returned as is, wrapped,
// so status.Code reads its code.
func (f *Fetcher) Fetch(ctx context.Context, req FetchRequest) (*FetchResult, error) {
	if err := req.check(); err != nil {
		return nil, err
	}

	f.store.mu.Lock()
	defer f.store.mu.Unlock()

	now := f.now().UTC()

	st, err := f.store.State()
	if err != nil {
		// A missing, damaged or newer state is an empty cache for the
		// fetcher. The next write replaces it. Without this, a tool that a
		// user downgrades could never get a policy again.
		log.Debugf("configsync: fetch starts from an empty cache: %v", err)
		st = &State{}
	}

	sameTenant := st.Tenant == req.TenantDomain
	var held string
	var highest uint64
	if sameTenant {
		highest = st.HighestRevision
		held = f.heldDigest(st)
	}

	rpcReq := policyv1.GetResolvedPolicyRequest_builder{
		Endpoint:    req.Endpoint,
		Labels:      req.Labels,
		Digest:      held,
		ToolName:    req.ToolName,
		ToolVersion: req.ToolVersion,
	}.Build()

	resp, err := f.transport.GetResolvedPolicy(ctx, rpcReq)
	if err != nil {
		return nil, f.fail(st, now, fmt.Errorf("configsync: fetch failed: %w", err))
	}

	switch resp.GetStatus() {
	case policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_NOT_MODIFIED:
		if held == "" || resp.GetDigest() != held {
			return nil, f.fail(st, now, fmt.Errorf("%w: not modified reply has digest %q, the cache holds %q",
				ErrDigestMismatch, resp.GetDigest(), held))
		}
		st.FetchedAt = now
		if err := f.store.writeState(st); err != nil {
			return nil, err
		}
		return &FetchResult{Status: FetchNotModified, Digest: st.Digest, Revision: st.Revision}, nil

	case policyv1.ResolvedPolicyStatus_RESOLVED_POLICY_STATUS_MODIFIED:
		return f.commitModified(st, now, req, resp, sameTenant, held, highest)

	default:
		return nil, f.fail(st, now, fmt.Errorf("%w: %v", ErrUnknownStatus, resp.GetStatus()))
	}
}

func (f *Fetcher) commitModified(st *State, now time.Time, req FetchRequest,
	resp *policyv1.GetResolvedPolicyResponse, sameTenant bool, held string, highest uint64,
) (*FetchResult, error) {
	raw := resp.GetResolvedPolicy()
	digest := resp.GetDigest()
	revision := resp.GetRevision()

	if got := Digest(raw); got != digest {
		return nil, f.fail(st, now, fmt.Errorf("%w: reply digest %q, bytes have %q", ErrDigestMismatch, digest, got))
	}

	// A lower revision can come from a replica that lags or from a replay.
	// The same revision with a new digest is accepted: the server can
	// resolve one revision again, for example after a label change.
	if revision < highest {
		return nil, f.fail(st, now, fmt.Errorf("%w: revision %d, highest %d", ErrLowerRevision, revision, highest))
	}

	if err := req.Validate(raw); err != nil {
		rejectErr := fmt.Errorf("%w: %w", ErrRejected, err)
		st.Rejected = &Rejection{Digest: digest, Revision: revision, Reason: err.Error(), At: now}
		return nil, f.fail(st, now, rejectErr)
	}

	next := State{
		Tenant:          req.TenantDomain,
		Digest:          digest,
		Revision:        revision,
		HighestRevision: max(highest, revision),
		StoredAt:        now,
		FetchedAt:       now,
		LastAttemptAt:   now,
	}

	// A policy of another tenant is no fallback: its header check fails.
	if sameTenant {
		switch {
		case held != "" && held != digest:
			next.PreviousDigest, next.PreviousRevision = held, st.Revision
		case st.PreviousDigest != digest:
			// The current file was lost or damaged, so the previous file
			// stays the fallback.
			next.PreviousDigest, next.PreviousRevision = st.PreviousDigest, st.PreviousRevision
		}
	}

	if sameTenant && req.CapabilitySources != nil {
		next.CapabilitySources = cleanSources(req.CapabilitySources(raw), digest)
	}

	if err := f.store.commit(&next, raw); err != nil {
		return nil, f.fail(st, now, err)
	}

	log.Debugf("configsync: committed policy %s revision %d", digest, revision)
	return &FetchResult{Status: FetchModified, Digest: digest, Revision: revision}, nil
}

// heldDigest returns the digest of the current policy file when the file
// passes its SHA-256 check. A lost or damaged file sends an empty digest, so
// the server sends the policy again and the cache repairs itself.
func (f *Fetcher) heldDigest(st *State) string {
	if !st.HasPolicy() {
		return ""
	}
	if _, err := f.store.ReadPolicy(st.Digest); err != nil {
		log.Warnf("configsync: current policy file is not usable, fetch sends an empty digest: %v", err)
		return ""
	}
	return st.Digest
}

// fail records a failed fetch in state.json and returns err. It writes no
// policy bytes.
func (f *Fetcher) fail(st *State, now time.Time, err error) error {
	st.LastAttemptAt = now
	st.LastError = err.Error()
	if werr := f.store.writeState(st); werr != nil {
		log.Warnf("configsync: cannot record the failed fetch: %v", werr)
	}
	return err
}

func cleanSources(in map[string]string, current string) map[string]string {
	out := map[string]string{}
	for capability, d := range in {
		if d == current {
			continue
		}
		if !validDigest.MatchString(d) {
			log.Warnf("configsync: capability %q has an invalid source digest %q, dropped", capability, d)
			continue
		}
		out[capability] = d
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *FetchRequest) check() error {
	switch {
	case r.TenantDomain == "":
		return fmt.Errorf("%w: tenant domain is required", ErrInvalidRequest)
	case r.Endpoint == nil:
		return fmt.Errorf("%w: endpoint identity is required", ErrInvalidRequest)
	case r.ToolName == "":
		return fmt.Errorf("%w: tool name is required", ErrInvalidRequest)
	case r.Validate == nil:
		return fmt.Errorf("%w: validate hook is required", ErrInvalidRequest)
	}
	return nil
}
