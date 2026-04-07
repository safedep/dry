package endpointsync

import (
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndpointIdentityResolver(t *testing.T) {
	t.Run("uses configured endpoint ID", func(t *testing.T) {
		resolver := NewEndpointIdentityResolver(
			WithEndpointID("my-machine"),
		)

		identity, err := resolver.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "my-machine", identity.GetIdentifier())
		assert.NotNil(t, identity.GetMetadata())
		assert.NotEqual(t, controltowerv1.EndpointOS_ENDPOINT_OS_UNSPECIFIED, identity.GetMetadata().GetOs())
		assert.NotEqual(t, controltowerv1.EndpointArch_ENDPOINT_ARCH_UNSPECIFIED, identity.GetMetadata().GetArch())
		assert.NotEmpty(t, identity.GetMetadata().GetHostname())
	})

	t.Run("falls back to hostname when no ID configured", func(t *testing.T) {
		resolver := NewEndpointIdentityResolver()

		identity, err := resolver.Resolve()
		require.NoError(t, err)
		assert.NotEmpty(t, identity.GetIdentifier())
		assert.NotNil(t, identity.GetMetadata())
	})

	t.Run("empty configured ID falls back to hostname", func(t *testing.T) {
		resolver := NewEndpointIdentityResolver(
			WithEndpointID(""),
		)

		identity, err := resolver.Resolve()
		require.NoError(t, err)
		assert.NotEmpty(t, identity.GetIdentifier())
	})
}
