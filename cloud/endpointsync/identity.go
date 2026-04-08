package endpointsync

import (
	"os"
	"runtime"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	"github.com/denisbrodbeck/machineid"
	"github.com/safedep/dry/log"
)

// EndpointIdentityResolver resolves the endpoint identity for sync.
type EndpointIdentityResolver interface {
	Resolve() (*controltowerv1.EndpointIdentity, error)
}

// EndpointIdentityOption configures the default identity resolver.
type EndpointIdentityOption func(*defaultEndpointIdentityResolver)

// WithEndpointID sets an operator-provided endpoint identifier.
// If not set or empty, the resolver falls back to hostname.
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
	identifier := r.configuredID

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	if identifier == "" {
		identifier = hostname
		log.Debugf("No endpoint ID configured, using hostname %q", hostname)
	}

	// ProtectedID returns an HMAC-SHA256 hash of the raw machine ID using
	// "safedep" as the app key. This is stable, unique per machine, and
	// does not expose the raw system UUID.
	mid, err := machineid.ProtectedID("safedep")
	if err != nil {
		log.Warnf("Failed to read machine ID: %v. Endpoint deduplication may be degraded.", err)
		mid = ""
	}

	return &controltowerv1.EndpointIdentity{
		Identifier: identifier,
		MachineId:  mid,
		Metadata: &controltowerv1.EndpointMetadata{
			Hostname: hostname,
			Os:       detectOS(),
			Arch:     detectArch(),
		},
	}, nil
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
