package cloud

import (
	"fmt"
	"net"
	"net/http"
	"os"

	drygrpc "github.com/safedep/dry/adapters/grpc"
	"google.golang.org/grpc"
)

const (
	defaultDataPlaneAddr    = "api.safedep.io:443"
	defaultControlPlaneAddr = "cloud.safedep.io:443"
	defaultPort             = "443"

	envDataPlaneAddr    = "SAFEDEP_CLOUD_DATA_ADDR"
	envControlPlaneAddr = "SAFEDEP_CLOUD_CONTROL_ADDR"

	tenantIDHeader = "x-tenant-id"
)

// Client is a connection to SafeDep Cloud.
type Client struct {
	conn *grpc.ClientConn
}

// NewDataPlaneClient creates a connection to api.safedep.io (API key auth).
func NewDataPlaneClient(name string, creds *Credentials) (*Client, error) {
	if creds == nil {
		return nil, fmt.Errorf("%w: credentials are required", ErrMissingCredentials)
	}

	apiKey, err := creds.GetAPIKey()
	if err != nil {
		return nil, err
	}

	tenantDomain, err := creds.GetTenantDomain()
	if err != nil {
		return nil, err
	}

	host, port := parseAddr(envOrDefault(envDataPlaneAddr, defaultDataPlaneAddr))

	headers := http.Header{}
	headers.Set(tenantIDHeader, tenantDomain)

	conn, err := drygrpc.GrpcClient(name, host, port, apiKey, headers, []grpc.DialOption{})
	if err != nil {
		return nil, fmt.Errorf("cloud: failed to create data plane connection: %w", err)
	}

	return &Client{conn: conn}, nil
}

// NewControlPlaneClient creates a connection to cloud.safedep.io (JWT auth).
func NewControlPlaneClient(name string, creds *Credentials) (*Client, error) {
	if creds == nil {
		return nil, fmt.Errorf("%w: credentials are required", ErrMissingCredentials)
	}

	token, err := creds.GetToken()
	if err != nil {
		return nil, err
	}

	tenantDomain, err := creds.GetTenantDomain()
	if err != nil {
		return nil, err
	}

	host, port := parseAddr(envOrDefault(envControlPlaneAddr, defaultControlPlaneAddr))

	headers := http.Header{}
	headers.Set(tenantIDHeader, tenantDomain)

	conn, err := drygrpc.GrpcClient(name, host, port, token, headers, []grpc.DialOption{})
	if err != nil {
		return nil, fmt.Errorf("cloud: failed to create control plane connection: %w", err)
	}

	return &Client{conn: conn}, nil
}

// Connection returns the underlying gRPC client connection.
func (c *Client) Connection() *grpc.ClientConn {
	return c.conn
}

// Close closes the connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// parseAddr splits a "host:port" address. If port is missing, defaults to 443.
func parseAddr(addr string) (host, port string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// No port in the address
		return addr, defaultPort
	}
	return host, port
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
