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
		assert.NotEmpty(t, identity.GetMachineId(), "machine_id should be populated")
		assert.NotNil(t, identity.GetMetadata())
		assert.NotEqual(t, controltowerv1.EndpointOS_ENDPOINT_OS_UNSPECIFIED, identity.GetMetadata().GetOs())
		assert.NotEqual(t, controltowerv1.EndpointArch_ENDPOINT_ARCH_UNSPECIFIED, identity.GetMetadata().GetArch())
		assert.NotEmpty(t, identity.GetMetadata().GetHostname())
	})

	t.Run("machine_id is stable across calls", func(t *testing.T) {
		resolver := NewEndpointIdentityResolver(WithEndpointID("test"))

		id1, err := resolver.Resolve()
		require.NoError(t, err)
		id2, err := resolver.Resolve()
		require.NoError(t, err)
		assert.Equal(t, id1.GetMachineId(), id2.GetMachineId())
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
