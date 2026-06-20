package stream

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestProtoBinarySerializer(t *testing.T) {
	record := &packagev1.PackageVersion{
		Package: &packagev1.Package{
			Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
			Name:      "test-package",
		},
		Version: "1.0.0",
	}

	t.Run("serialized record should be deserializable", func(t *testing.T) {
		serializer, err := NewProtoBinarySerializer[*packagev1.PackageVersion]()
		require.NoError(t, err)

		serialized, err := serializer.Serialize(record)
		require.NoError(t, err)

		deserialized := &packagev1.PackageVersion{}
		require.NoError(t, serializer.Deserialize(serialized, deserialized))

		assert.True(t, proto.Equal(record, deserialized))
	})

	t.Run("output is binary proto wire format, not JSON", func(t *testing.T) {
		serializer, err := NewProtoBinarySerializer[*packagev1.PackageVersion]()
		require.NoError(t, err)

		serialized, err := serializer.Serialize(record)
		require.NoError(t, err)

		// Must match canonical proto.Marshal bytes (the contract wire format)...
		want, err := proto.Marshal(record)
		require.NoError(t, err)
		assert.Equal(t, want, serialized)

		// ...and must NOT be the ProtoJSON rendering.
		assert.NotEqual(t, byte('{'), serialized[0])
	})

	t.Run("deserialize rejects malformed bytes", func(t *testing.T) {
		serializer, err := NewProtoBinarySerializer[*packagev1.PackageVersion]()
		require.NoError(t, err)

		err = serializer.Deserialize([]byte{0xff, 0xff, 0xff}, &packagev1.PackageVersion{})
		require.Error(t, err)
	})
}
