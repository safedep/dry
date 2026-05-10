package endpointsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	"github.com/denisbrodbeck/machineid"
	"github.com/safedep/dry/log"
)

const endpointIdentityHMACKey = "safedep"

// EndpointIdentityResolver resolves the endpoint identity for sync.
type EndpointIdentityResolver interface {
	Resolve() (*controltowerv1.EndpointIdentity, error)
}

// EndpointIdentityOption configures the default identity resolver.
type EndpointIdentityOption func(*defaultEndpointIdentityResolver)

// WithEndpointID sets an operator-provided endpoint identifier.
//
// When set, this value becomes the source of truth for endpoint identity:
// it is used as the human-readable Identifier and the MachineId is derived
// from it via HMAC-SHA256 instead of being read from the host. This yields
// a stable identity across machines, which is required for ephemeral
// environments such as CI/CD runners where the system machine ID changes
// every run.
//
// If not set or empty, the resolver falls back to hostname for the
// Identifier and uses the host's machine ID for MachineId.
func WithEndpointID(id string) EndpointIdentityOption {
	return func(r *defaultEndpointIdentityResolver) {
		r.configuredID = id
	}
}

// NewEndpointIdentityResolver creates a default identity resolver.
func NewEndpointIdentityResolver(opts ...EndpointIdentityOption) EndpointIdentityResolver {
	r := &defaultEndpointIdentityResolver{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type defaultEndpointIdentityResolver struct {
	configuredID string
}

func (r *defaultEndpointIdentityResolver) Resolve() (*controltowerv1.EndpointIdentity, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	var identifier, machineID string
	if r.configuredID != "" {
		// Operator has taken explicit ownership of identity. Derive a stable
		// MachineId from the configured ID so ephemeral environments (e.g.
		// CI/CD runners) don't churn endpoint records on every run.
		identifier = r.configuredID
		machineID = hmacEndpointID(r.configuredID)
	} else {
		log.Debugf("No endpoint ID configured, using hostname %q", hostname)
		identifier = hostname

		// ProtectedID returns an HMAC-SHA256 hash of the raw machine ID using
		// "safedep" as the app key. Stable per machine, does not expose the
		// raw system UUID.
		mid, err := machineid.ProtectedID(endpointIdentityHMACKey)
		if err != nil {
			return nil, fmt.Errorf("failed to read machine ID: %w", err)
		}
		machineID = mid
	}

	return &controltowerv1.EndpointIdentity{
		Identifier: identifier,
		MachineId:  machineID,
		Metadata: &controltowerv1.EndpointMetadata{
			Hostname: hostname,
			Os:       detectOS(),
			Arch:     detectArch(),
		},
	}, nil
}

func hmacEndpointID(id string) string {
	mac := hmac.New(sha256.New, []byte(endpointIdentityHMACKey))
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

func detectOS() controltowerv1.EndpointOS {
	switch runtime.GOOS {
	case "darwin":
		return controltowerv1.EndpointOS_ENDPOINT_OS_DARWIN
	case "linux":
		return controltowerv1.EndpointOS_ENDPOINT_OS_LINUX
	case "windows":
		return controltowerv1.EndpointOS_ENDPOINT_OS_WINDOWS
	default:
		return controltowerv1.EndpointOS_ENDPOINT_OS_UNSPECIFIED
	}
}

func detectArch() controltowerv1.EndpointArch {
	switch runtime.GOARCH {
	case "amd64":
		return controltowerv1.EndpointArch_ENDPOINT_ARCH_AMD64
	case "arm64":
		return controltowerv1.EndpointArch_ENDPOINT_ARCH_ARM64
	case "arm":
		return controltowerv1.EndpointArch_ENDPOINT_ARCH_ARM
	default:
		return controltowerv1.EndpointArch_ENDPOINT_ARCH_UNSPECIFIED
	}
}
