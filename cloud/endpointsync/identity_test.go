package endpointsync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	controltowerv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/controltower/v1"
	"github.com/denisbrodbeck/machineid"
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

	t.Run("machine_id is derived from configured ID via HMAC-SHA256", func(t *testing.T) {
		const configuredID = "ci-safedep-vet"

		mac := hmac.New(sha256.New, []byte("safedep"))
		mac.Write([]byte(configuredID))
		expected := hex.EncodeToString(mac.Sum(nil))

		identity, err := NewEndpointIdentityResolver(WithEndpointID(configuredID)).Resolve()
		require.NoError(t, err)
		assert.Equal(t, expected, identity.GetMachineId())
	})

	t.Run("same configured ID yields same machine_id across resolver instances", func(t *testing.T) {
		id1, err := NewEndpointIdentityResolver(WithEndpointID("stable-ci")).Resolve()
		require.NoError(t, err)
		id2, err := NewEndpointIdentityResolver(WithEndpointID("stable-ci")).Resolve()
		require.NoError(t, err)
		assert.Equal(t, id1.GetMachineId(), id2.GetMachineId())
	})

	t.Run("different configured IDs yield different machine_id", func(t *testing.T) {
		id1, err := NewEndpointIdentityResolver(WithEndpointID("endpoint-a")).Resolve()
		require.NoError(t, err)
		id2, err := NewEndpointIdentityResolver(WithEndpointID("endpoint-b")).Resolve()
		require.NoError(t, err)
		assert.NotEqual(t, id1.GetMachineId(), id2.GetMachineId())
	})

	t.Run("machine_id derived from configured ID does not depend on system machine ID", func(t *testing.T) {
		systemID, err := machineid.ProtectedID("safedep")
		require.NoError(t, err)

		identity, err := NewEndpointIdentityResolver(WithEndpointID("explicit")).Resolve()
		require.NoError(t, err)
		assert.NotEqual(t, systemID, identity.GetMachineId())
	})

	t.Run("without configured ID, machine_id matches system ProtectedID", func(t *testing.T) {
		expected, err := machineid.ProtectedID("safedep")
		require.NoError(t, err)

		identity, err := NewEndpointIdentityResolver().Resolve()
		require.NoError(t, err)
		assert.Equal(t, expected, identity.GetMachineId())
	})
}
