package stream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/s2-streamstore/s2-sdk-go/s2"
)

// s2StreamMatcher translates a StreamAccessRequest into the S2 ResourceSet
// used for the token's Streams scope.
//
// When request.Scope is nil, the matcher is an exact match on Stream.ID().
// When request.Scope is non-nil, the matcher is a prefix built from the
// populated fields, joined with the same ':' separator used by Stream.ID().
func s2StreamMatcher(request StreamAccessRequest) (*s2.ResourceSet, error) {
	if request.Scope == nil {
		id, err := request.Stream.ID()
		if err != nil {
			return nil, fmt.Errorf("stream matcher: %w", err)
		}
		return &s2.ResourceSet{Exact: s2.Ptr(id)}, nil
	}
	return &s2.ResourceSet{Prefix: s2.Ptr(s2ScopePrefix(*request.Scope))}, nil
}

// s2ScopePrefix builds the colon-delimited prefix for a StreamScope.
// Rules:
//   - If TenantID is set, it is the first segment and gets a trailing ':'.
//   - If Namespace is set, it is appended with a trailing ':'.
//   - NamePrefix is appended as-is (no trailing ':' because it is a partial
//     stream-name match).
//
// Examples: {t1} -> "t1:", {t1, ns1} -> "t1:ns1:",
// {t1, ns1, foo} -> "t1:ns1:foo", {NamePrefix: foo} -> "foo".
func s2ScopePrefix(scope StreamScope) string {
	out := ""
	if scope.TenantID != "" {
		out += scope.TenantID + ":"
	}
	if scope.Namespace != "" {
		out += scope.Namespace + ":"
	}
	if scope.NamePrefix != "" {
		out += scope.NamePrefix
	}
	return out
}

// s2OpsForRole maps the coarse StreamAccessRole to the concrete S2 ops it
// implies. Control-plane ops (trim, retain, reconfigure-stream, etc.) are
// deliberately not granted by any role. S2 Operation values are untyped
// string constants; the return type is []string.
func s2OpsForRole(role StreamAccessRole) []string {
	switch role {
	case StreamAccessRead:
		return []string{s2.OperationRead, s2.OperationCheckTail}
	case StreamAccessWrite:
		return []string{s2.OperationAppend}
	case StreamAccessReadWrite:
		return []string{s2.OperationRead, s2.OperationCheckTail, s2.OperationAppend}
	default:
		return nil
	}
}

// S2ControlPlaneConfig configures the S2 control plane. AdminApiKey must
// have basin-level token-issuance privilege. It is intentionally distinct
// from the data-plane key in S2StreamProviderConfig so writer services
// never carry minter credentials.
type S2ControlPlaneConfig struct {
	AdminApiKey string
}

// DefaultS2ControlPlaneConfig reads admin credentials from the environment.
func DefaultS2ControlPlaneConfig() S2ControlPlaneConfig {
	return S2ControlPlaneConfig{
		AdminApiKey: os.Getenv("STREAM_PROVIDER_S2_ADMIN_API_KEY"),
	}
}

// errS2AccessTokenNotFound is the sentinel the real s2 client wrapper returns
// when the provider reports that an access token no longer exists. Used by
// DeleteStreamAccess to implement idempotent revocation.
var errS2AccessTokenNotFound = errors.New("s2 access token not found")

// s2IssueInput is the library-local request shape handed to the test-seam
// s2AccessTokenClient. Keeping this struct inside the stream package (rather
// than leaking *s2.IssueAccessTokenArgs) lets tests construct simple fakes.
type s2IssueInput struct {
	ID                string
	ExpiresAt         time.Time
	BasinMatch        *s2.ResourceSet
	StreamMatch       *s2.ResourceSet
	Ops               []string
	AutoPrefixStreams bool
}

// s2IssueOutput captures just the fields we forward to callers. ExpiresAt
// is what the provider returned, which may be normalized.
type s2IssueOutput struct {
	Token     string
	ExpiresAt time.Time
}

// s2AccessTokenClient is the unexported test seam. The real implementation
// (s2AccessTokenClientImpl, below) wraps *s2.Client. Unit tests inject a fake
// satisfying this interface, keeping translation/validation logic verifiable
// without S2 credentials.
type s2AccessTokenClient interface {
	IssueAccessToken(ctx context.Context, in *s2IssueInput) (*s2IssueOutput, error)

	// RevokeAccessToken returns errS2AccessTokenNotFound when the provider
	// reports the token does not exist (already revoked or never issued).
	// All other errors propagate unchanged.
	RevokeAccessToken(ctx context.Context, id string) error
}

// s2AccessTokenClientImpl is the real implementation of s2AccessTokenClient
// backed by *s2.Client.
type s2AccessTokenClientImpl struct {
	client *s2.Client
}

func (c *s2AccessTokenClientImpl) IssueAccessToken(ctx context.Context, in *s2IssueInput) (*s2IssueOutput, error) {
	expiresAt := in.ExpiresAt
	resp, err := c.client.AccessTokens.Issue(ctx, s2.IssueAccessTokenArgs{
		ID: s2.AccessTokenID(in.ID),
		Scope: s2.AccessTokenScope{
			Basins:  in.BasinMatch,
			Streams: in.StreamMatch,
			Ops:     in.Ops,
		},
		AutoPrefixStreams: in.AutoPrefixStreams,
		ExpiresAt:         &expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &s2IssueOutput{Token: resp.AccessToken, ExpiresAt: expiresAt}, nil
}

func (c *s2AccessTokenClientImpl) RevokeAccessToken(ctx context.Context, id string) error {
	err := c.client.AccessTokens.Revoke(ctx, s2.RevokeAccessTokenArgs{ID: s2.AccessTokenID(id)})
	if err == nil {
		return nil
	}
	// S2's HTTP SDK surfaces *s2.S2Error with an HTTP-style Status field.
	// Map 404 to our sentinel so the caller can implement idempotency.
	var s2Err *s2.S2Error
	if errors.As(err, &s2Err) && s2Err.Status == 404 {
		return errS2AccessTokenNotFound
	}
	return err
}

// s2StreamLifecycleClient is a separate seam for basin-level stream CRUD.
type s2StreamLifecycleClient interface {
	CreateStream(ctx context.Context, basin, name string) error
	DeleteStream(ctx context.Context, basin, name string) error
}

// errS2StreamNotFound is mapped from HTTP 404 to implement idempotent delete.
var errS2StreamNotFound = errors.New("s2 stream not found")

// errS2StreamAlreadyExists is mapped from HTTP 409 to implement idempotent create.
var errS2StreamAlreadyExists = errors.New("s2 stream already exists")

// s2StreamLifecycleClientImpl is the real implementation of s2StreamLifecycleClient
// backed by *s2.Client.
type s2StreamLifecycleClientImpl struct {
	client *s2.Client
}

func (c *s2StreamLifecycleClientImpl) CreateStream(ctx context.Context, basin, name string) error {
	_, err := c.client.Basin(basin).Streams.Create(ctx, s2.CreateStreamArgs{
		Stream: s2.StreamName(name),
	})
	if err == nil {
		return nil
	}
	// 409 Conflict signals "already exists" on S2's HTTP API.
	var s2Err *s2.S2Error
	if errors.As(err, &s2Err) && s2Err.Status == 409 {
		return errS2StreamAlreadyExists
	}
	return err
}

func (c *s2StreamLifecycleClientImpl) DeleteStream(ctx context.Context, basin, name string) error {
	err := c.client.Basin(basin).Streams.Delete(ctx, s2.StreamName(name))
	if err == nil {
		return nil
	}
	var s2Err *s2.S2Error
	if errors.As(err, &s2Err) && s2Err.Status == 404 {
		return errS2StreamNotFound
	}
	return err
}

// s2StreamControlPlane satisfies StreamControlPlane (both lifecycle and
// access-issuer). The public constructors return it typed narrowly.
type s2StreamControlPlane struct {
	client        *s2.Client
	tokens        s2AccessTokenClient
	streams       s2StreamLifecycleClient
	basinResolver S2BasinResolver
	// now is time.Now in production; tests can override.
	now func() time.Time
}

var _ StreamControlPlane = (*s2StreamControlPlane)(nil)

// NewS2StreamControlPlane returns an S2 control plane (lifecycle + issuer).
func NewS2StreamControlPlane(config S2ControlPlaneConfig, basinResolver S2BasinResolver) (StreamControlPlane, error) {
	cp, err := newS2StreamControlPlane(config, basinResolver)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

// NewS2StreamAccessIssuer returns just the access-issuer half, typed
// narrowly so minter services do not depend on lifecycle methods they
// should not call.
func NewS2StreamAccessIssuer(config S2ControlPlaneConfig, basinResolver S2BasinResolver) (StreamAccessIssuer, error) {
	cp, err := newS2StreamControlPlane(config, basinResolver)
	if err != nil {
		return nil, err
	}
	return cp, nil
}

func newS2StreamControlPlane(config S2ControlPlaneConfig, basinResolver S2BasinResolver) (*s2StreamControlPlane, error) {
	if config.AdminApiKey == "" {
		return nil, fmt.Errorf("S2 admin API key is not set")
	}
	if basinResolver == nil {
		return nil, fmt.Errorf("basin resolver is nil")
	}
	client := s2.New(config.AdminApiKey, nil)
	return &s2StreamControlPlane{
		client:        client,
		tokens:        &s2AccessTokenClientImpl{client: client},
		streams:       &s2StreamLifecycleClientImpl{client: client},
		basinResolver: basinResolver,
		now:           time.Now,
	}, nil
}

func (s *s2StreamControlPlane) CreateStream(ctx context.Context, stream Stream) error {
	id, err := stream.ID()
	if err != nil {
		return fmt.Errorf("s2 control: create stream: %w", err)
	}
	basin, err := s.basinResolver.GetBasin(ctx, stream.Namespace, stream.TenantID)
	if err != nil {
		return fmt.Errorf("s2 control: resolve basin: %w", err)
	}
	err = s.streams.CreateStream(ctx, basin, id)
	if err == nil || errors.Is(err, errS2StreamAlreadyExists) {
		return nil
	}
	return fmt.Errorf("s2 control: create stream: %w", err)
}

func (s *s2StreamControlPlane) DeleteStream(ctx context.Context, stream Stream) error {
	id, err := stream.ID()
	if err != nil {
		return fmt.Errorf("s2 control: delete stream: %w", err)
	}
	basin, err := s.basinResolver.GetBasin(ctx, stream.Namespace, stream.TenantID)
	if err != nil {
		return fmt.Errorf("s2 control: resolve basin: %w", err)
	}
	err = s.streams.DeleteStream(ctx, basin, id)
	if err == nil || errors.Is(err, errS2StreamNotFound) {
		return nil
	}
	return fmt.Errorf("s2 control: delete stream: %w", err)
}

func (s *s2StreamControlPlane) CreateStreamAccess(ctx context.Context, request StreamAccessRequest) (*StreamAccess, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}

	expiry, err := normalizeExpiry(request.Expiry)
	if err != nil {
		// Defensive — Validate already checked, but make the invariant explicit.
		return nil, err
	}

	// Resolve basin using the scope if present, else the single Stream.
	var (
		basinNamespace string
		basinTenant    string
	)
	if request.Scope != nil {
		basinNamespace = request.Scope.Namespace
		basinTenant = request.Scope.TenantID
	} else {
		basinNamespace = request.Stream.Namespace
		basinTenant = request.Stream.TenantID
	}
	basin, err := s.basinResolver.GetBasin(ctx, basinNamespace, basinTenant)
	if err != nil {
		return nil, fmt.Errorf("s2 control: resolve basin: %w", err)
	}

	streamMatch, err := s2StreamMatcher(request)
	if err != nil {
		return nil, fmt.Errorf("s2 control: build stream matcher: %w", err)
	}

	accessID := uuid.NewString()
	expiresAt := s.now().Add(expiry)

	out, err := s.tokens.IssueAccessToken(ctx, &s2IssueInput{
		ID:          accessID,
		ExpiresAt:   expiresAt,
		BasinMatch:  &s2.ResourceSet{Exact: s2.Ptr(basin)},
		StreamMatch: streamMatch,
		Ops:         s2OpsForRole(request.Access),
	})
	if err != nil {
		return nil, fmt.Errorf("s2 control: issue token: %w", err)
	}

	return &StreamAccess{
		AccessID:  accessID,
		Token:     out.Token,
		ExpiresAt: out.ExpiresAt,
	}, nil
}

func (s *s2StreamControlPlane) DeleteStreamAccess(ctx context.Context, accessID string) error {
	if accessID == "" {
		return ErrInvalidAccessID
	}
	err := s.tokens.RevokeAccessToken(ctx, accessID)
	if err == nil {
		return nil
	}
	if errors.Is(err, errS2AccessTokenNotFound) {
		// Idempotent: already revoked or never existed.
		return nil
	}
	return fmt.Errorf("s2 control: revoke token: %w", err)
}
