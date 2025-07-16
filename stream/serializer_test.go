package stream

import (
	"testing"

	packagev1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/messages/package/v1"
	"github.com/stretchr/testify/assert"
)

func TestProtoJsonSerializer(t *testing.T) {
	t.Run("serialized record should be deserializable", func(t *testing.T) {
		serializer, err := NewProtoJsonSerializer[*packagev1.PackageVersion]()
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}

		record := &packagev1.PackageVersion{
			Package: &packagev1.Package{
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
				Name:      "test-package",
			},
			Version: "1.0.0",
		}

		serialized, err := serializer.Serialize(record)
		if err != nil {
			t.Fatalf("failed to serialize record: %v", err)
		}

		deserialized := &packagev1.PackageVersion{}
		err = serializer.Deserialize(serialized, deserialized)
		if err != nil {
			t.Fatalf("failed to deserialize record: %v", err)
		}

		assert.Equal(t, record, deserialized)
	})

	t.Run("serialized record must be valid JSON", func(t *testing.T) {
		serializer, err := NewProtoJsonSerializer[*packagev1.PackageVersion]()
		if err != nil {
			t.Fatalf("failed to create serializer: %v", err)
		}

		record := &packagev1.PackageVersion{
			Package: &packagev1.Package{
				Ecosystem: packagev1.Ecosystem_ECOSYSTEM_NPM,
				Name:      "test-package",
			},
			Version: "1.0.0",
		}

		serialized, err := serializer.Serialize(record)
		if err != nil {
			t.Fatalf("failed to serialize record: %v", err)
		}

		assert.JSONEq(t, `{"package":{"ecosystem":"ECOSYSTEM_NPM","name":"test-package"},"version":"1.0.0"}`,
			string(serialized))
	})
}
