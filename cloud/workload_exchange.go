package cloud

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"buf.build/gen/go/safedep/api/grpc/go/safedep/services/controltower/v1/controltowerv1grpc"
	messagesv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/services/controltower/v1"
	"github.com/safedep/dry/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	// workloadExchangeTimeout bounds one exchange: the ID token request and
	// the ExchangeToken call. An RPC that waits for a new credential waits at
	// most this long.
	workloadExchangeTimeout = 30 * time.Second

	// workloadRefreshMargin is how long before expiry the refreshing
	// credential exchanges again. A credential that lives less than twice the
	// margin refreshes at half its life.
	workloadRefreshMargin = 5 * time.Minute

	// workloadRefreshRetryDelay is how long a failed exchange holds before the
	// next one. Without it, every RPC against a denied or hung exchange fetches
	// an ID token and calls ExchangeToken again.
	workloadRefreshRetryDelay = 30 * time.Second
)

// ErrWorkloadCredentialRevoked is returned by a RefreshingWorkloadCredential
// after Revoke.
var ErrWorkloadCredentialRevoked = errors.New("cloud: the workload credential is revoked")

// WorkloadExchangeRequest holds what a workload asks for in an exchange.
type WorkloadExchangeRequest struct {
	// TenantDomain is the tenant that the credential is for. Required.
	TenantDomain string

	// Lifetime is the life that the workload asks for, at most 6 hours. Zero
	// takes the maximum of the workload trust rule.
	Lifetime time.Duration

	// ToolName and ToolVersion name the tool that asks, as in "vet". The
	// server writes them to its log of the exchange. Both are optional.
	ToolName    string
	ToolVersion string
}

// WorkloadCredential is a short-lived credential from ExchangeToken. Send
// AccessToken as an API key. Do not write AccessToken to a log. String omits
// it.
type WorkloadCredential struct {
	AccessToken     string
	ExpiresAt       time.Time
	TenantDomain    string
	EndpointID      string
	CredentialID    string
	DisplayIdentity string
}

// String describes the credential without its access token.
func (c WorkloadCredential) String() string {
	return fmt.Sprintf("workload credential %s for %s in %s, expires at %s",
		c.CredentialID, c.DisplayIdentity, c.TenantDomain, c.ExpiresAt.Format(time.RFC3339))
}

// GoString keeps the access token out of %#v.
func (c WorkloadCredential) GoString() string {
	return c.String()
}

// WorkloadExchangeClient trades a workload identity token for a workload
// credential, and revokes a workload credential. Every SafeDep tool uses it,
// so that each tool exchanges the same way.
type WorkloadExchangeClient struct {
	client controltowerv1grpc.WorkloadIdentityServiceClient

	// dial, when set, opens a connection for one call. An exchange runs about
	// once in 6 hours, so a connection per call costs little, and no idle
	// connection outlives the resolver that made it.
	dial func() (*grpc.ClientConn, error)
}

// NewWorkloadExchangeClient returns an exchange client on conn. The
// WorkloadIdentityService is keyless: conn must send no API key and no JWT,
// or the server refuses the call.
func NewWorkloadExchangeClient(conn grpc.ClientConnInterface) *WorkloadExchangeClient {
	return &WorkloadExchangeClient{client: controltowerv1grpc.NewWorkloadIdentityServiceClient(conn)}
}

func (c *WorkloadExchangeClient) call(fn func(controltowerv1grpc.WorkloadIdentityServiceClient) error) error {
	if c.dial == nil {
		return fn(c.client)
	}

	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Warnf("cloud: close the workload identity connection: %v", err)
		}
	}()

	return fn(controltowerv1grpc.NewWorkloadIdentityServiceClient(conn))
}

// Exchange gets a new token from source and trades it for a workload
// credential with the grant TOOL. It gets a new token for each exchange,
// because the server accepts a token one time only and a GitHub Actions ID
// token lives 5 minutes.
func (c *WorkloadExchangeClient) Exchange(ctx context.Context, source WorkloadIdentitySource,
	req WorkloadExchangeRequest) (WorkloadCredential, error) {
	if source == nil {
		return WorkloadCredential{}, fmt.Errorf("%w: workload identity source is required", ErrMissingCredentials)
	}
	if req.TenantDomain == "" {
		return WorkloadCredential{}, fmt.Errorf("%w: tenant domain is required", ErrMissingCredentials)
	}

	token, err := source.Token(ctx, WorkloadIdentityAudience)
	if err != nil {
		return WorkloadCredential{}, fmt.Errorf("cloud: get the workload identity token: %w", err)
	}

	exchangeReq := &controltowerv1.ExchangeTokenRequest{
		SubjectToken:     token.Value,
		SubjectTokenType: token.Type,
		TenantDomain:     req.TenantDomain,
		RequestedGrant:   messagesv1.CredentialGrant_CREDENTIAL_GRANT_TOOL,
		ToolName:         req.ToolName,
		ToolVersion:      req.ToolVersion,
	}
	if req.Lifetime > 0 {
		exchangeReq.RequestedLifetime = durationpb.New(req.Lifetime)
	}

	var res *controltowerv1.ExchangeTokenResponse
	err = c.call(func(client controltowerv1grpc.WorkloadIdentityServiceClient) error {
		var err error
		res, err = client.ExchangeToken(ctx, exchangeReq)
		return err
	})
	if err != nil {
		return WorkloadCredential{}, fmt.Errorf("cloud: exchange the workload identity token: %w", err)
	}
	if res.GetAccessToken() == "" || res.GetExpiresAt() == nil {
		return WorkloadCredential{}, errors.New("cloud: the exchange returned no credential")
	}

	tenantDomain := res.GetTenantDomain()
	if tenantDomain == "" {
		tenantDomain = req.TenantDomain
	}

	return WorkloadCredential{
		AccessToken:     res.GetAccessToken(),
		ExpiresAt:       res.GetExpiresAt().AsTime(),
		TenantDomain:    tenantDomain,
		EndpointID:      res.GetEndpointId(),
		CredentialID:    res.GetCredentialId(),
		DisplayIdentity: res.GetDisplayIdentity(),
	}, nil
}

// Revoke revokes cred at once. The server also succeeds for a credential that
// is already revoked or expired.
func (c *WorkloadExchangeClient) Revoke(ctx context.Context, cred WorkloadCredential) error {
	err := c.call(func(client controltowerv1grpc.WorkloadIdentityServiceClient) error {
		_, err := client.RevokeToken(ctx, &controltowerv1.RevokeTokenRequest{
			AccessToken:  cred.AccessToken,
			TenantDomain: cred.TenantDomain,
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("cloud: revoke the workload credential %s: %w", cred.CredentialID, err)
	}
	return nil
}

// RefreshingWorkloadCredential is a gRPC per-RPC credential that holds a
// workload credential and exchanges again before it expires. It sends the
// credential in the authorization header and the tenant in x-tenant-id.
//
// It exchanges on the first RPC, not at construction. One exchange runs at a
// time, outside the lock, so a slow exchange never blocks an RPC that holds a
// valid credential. A caller with no valid credential waits for the exchange
// or for its own context. A failed exchange holds for
// workloadRefreshRetryDelay: callers with no valid credential get its error,
// and callers with a valid credential keep it until it expires. So a job that
// can no longer get an ID token works to the end of the credential life.
type RefreshingWorkloadCredential struct {
	client *WorkloadExchangeClient
	source WorkloadIdentitySource
	req    WorkloadExchangeRequest
	now    func() time.Time

	mu         sync.Mutex
	current    WorkloadCredential
	obtainedAt time.Time
	inflight   *workloadExchange
	lastErr    error
	retryAfter time.Time
	revoked    bool
}

// workloadExchange is one exchange in flight. done closes when token and err
// are set.
type workloadExchange struct {
	done  chan struct{}
	token string
	err   error
}

// NewRefreshingWorkloadCredential returns a credential that exchanges a token
// from source with client.
func NewRefreshingWorkloadCredential(client *WorkloadExchangeClient, source WorkloadIdentitySource,
	req WorkloadExchangeRequest) *RefreshingWorkloadCredential {
	return &RefreshingWorkloadCredential{
		client: client,
		source: source,
		req:    req,
		now:    time.Now,
	}
}

// TenantDomain returns the tenant that the credential is for.
func (r *RefreshingWorkloadCredential) TenantDomain() string {
	return r.req.TenantDomain
}

// String describes the credential without its access token.
func (r *RefreshingWorkloadCredential) String() string {
	return fmt.Sprintf("refreshing workload credential for %s", r.req.TenantDomain)
}

// GoString keeps the access token out of %#v.
func (r *RefreshingWorkloadCredential) GoString() string {
	return r.String()
}

// AccessToken returns a valid access token. It starts an exchange when the
// holder has no credential, or when the credential is close to its expiry.
func (r *RefreshingWorkloadCredential) AccessToken(ctx context.Context) (string, error) {
	r.mu.Lock()

	if r.revoked {
		r.mu.Unlock()
		return "", ErrWorkloadCredentialRevoked
	}

	now := r.now()
	valid := r.validAt(now)
	if valid && now.Before(r.refreshAt()) {
		token := r.current.AccessToken
		r.mu.Unlock()
		return token, nil
	}

	exchange := r.inflight
	if exchange == nil && now.Before(r.retryAfter) {
		token, err := r.current.AccessToken, r.lastErr
		r.mu.Unlock()
		if valid {
			return token, nil
		}
		return "", err
	}

	if exchange == nil {
		exchange = r.startExchange()
	}

	if valid {
		token := r.current.AccessToken
		r.mu.Unlock()
		return token, nil
	}
	r.mu.Unlock()

	select {
	case <-exchange.done:
		return exchange.token, exchange.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// startExchange runs one exchange in the background. The caller holds r.mu.
func (r *RefreshingWorkloadCredential) startExchange() *workloadExchange {
	exchange := &workloadExchange{done: make(chan struct{})}
	r.inflight = exchange

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), workloadExchangeTimeout)
		cred, err := r.client.Exchange(ctx, r.source, r.req)
		cancel()

		r.mu.Lock()
		defer close(exchange.done)
		defer r.mu.Unlock()

		r.inflight = nil
		now := r.now()

		if err != nil {
			r.lastErr = err
			r.retryAfter = now.Add(workloadRefreshRetryDelay)
			if r.validAt(now) {
				log.Warnf("cloud: refresh of the workload credential failed, the current one stays valid until %s: %v",
					r.current.ExpiresAt.Format(time.RFC3339), err)
			}
			exchange.err = err
			return
		}

		// Keep a credential that arrives after Revoke, so that Revoke can
		// revoke it.
		r.current = cred
		r.obtainedAt = now
		r.lastErr = nil
		r.retryAfter = time.Time{}

		if r.revoked {
			exchange.err = ErrWorkloadCredentialRevoked
			return
		}
		exchange.token = cred.AccessToken
	}()

	return exchange
}

func (r *RefreshingWorkloadCredential) validAt(now time.Time) bool {
	return r.current.AccessToken != "" && now.Before(r.current.ExpiresAt)
}

func (r *RefreshingWorkloadCredential) refreshAt() time.Time {
	margin := workloadRefreshMargin
	if life := r.current.ExpiresAt.Sub(r.obtainedAt); life < 2*margin {
		margin = life / 2
	}
	return r.current.ExpiresAt.Add(-margin)
}

// Revoke revokes the current credential. Later calls to AccessToken fail with
// ErrWorkloadCredentialRevoked, and no new exchange starts. Revoke waits for
// an exchange in flight, or for ctx, so that it revokes the credential of that
// exchange too. When Revoke fails, a later call tries again. A credential that
// an earlier refresh replaced is not revoked: it expires within the refresh
// margin.
func (r *RefreshingWorkloadCredential) Revoke(ctx context.Context) error {
	r.mu.Lock()
	r.revoked = true
	exchange := r.inflight
	r.mu.Unlock()

	if exchange != nil {
		select {
		case <-exchange.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.mu.Lock()
	cred := r.current
	r.mu.Unlock()

	if cred.AccessToken == "" {
		return nil
	}
	if err := r.client.Revoke(ctx, cred); err != nil {
		return err
	}

	r.mu.Lock()
	if r.current.AccessToken == cred.AccessToken {
		r.current = WorkloadCredential{}
	}
	r.mu.Unlock()
	return nil
}

// GetRequestMetadata implements credentials.PerRPCCredentials.
func (r *RefreshingWorkloadCredential) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	token, err := r.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"authorization": token,
		tenantIDHeader:  r.req.TenantDomain,
	}, nil
}

// RequireTransportSecurity implements credentials.PerRPCCredentials.
func (r *RefreshingWorkloadCredential) RequireTransportSecurity() bool {
	return true
}
