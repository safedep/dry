// Package natstest holds test fixtures for the JetStream transport: a
// connection to a NATS test server, two private feeds and their events.
package natstest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/natsjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// URLEnv names the variable with the URL of a NATS server with JetStream.
const URLEnv = "DRY_NATS_TEST_URL"

const (
	ObservationFeed = "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"
	MaintainerFeed  = "safedep.events.private.packageregistry.v1.MaintainerObservationEvent"
	// Tenant is the tenant of the tests that need no server.
	Tenant = "tenant-a"
)

// Server is the part of the shared test server that one test owns: a stream
// name and a tenant that no other test uses.
type Server struct {
	JS     jetstream.JetStream
	Stream string
	Tenant string
}

// Connect connects to the server at DRY_NATS_TEST_URL. It skips the test when
// the variable is not set. The test cleanup deletes the stream of the test, with
// its consumers, and closes the connection.
func Connect(t testing.TB) Server {
	t.Helper()

	url := os.Getenv(URLEnv)
	if url == "" {
		t.Skipf("NATS test skipped: set %s to a NATS server with JetStream, for example "+
			"docker run -d --name dry-nats -p 4222:4222 nats:2.12.6-alpine -js and %s=nats://127.0.0.1:4222",
			URLEnv, URLEnv)
	}

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	suffix := randomSuffix(t)
	srv := Server{JS: js, Stream: streamName(t.Name(), suffix), Tenant: "t-" + suffix}

	t.Cleanup(func() {
		// t.Context is done when the cleanup runs.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := js.DeleteStream(ctx, srv.Stream)
		if err != nil && !errors.Is(err, jetstream.ErrStreamNotFound) {
			assert.NoError(t, err, "delete test stream")
		}
	})

	return srv
}

// StreamConfig is a stream over both feeds for the given tenants, or for the
// tenant of the test when none is given. The subjects of each test differ, so
// tests can share one server.
func (s Server) StreamConfig(tenants ...string) natsjs.StreamConfig {
	if len(tenants) == 0 {
		tenants = []string{s.Tenant}
	}
	var subjects []string
	for _, tenant := range tenants {
		subjects = append(subjects, ObservationFeed+"."+tenant, MaintainerFeed+"."+tenant)
	}

	return natsjs.StreamConfig{
		Name:     s.Stream,
		Subjects: subjects,
		MaxAge:   time.Hour,
		MaxBytes: 1 << 24,
	}
}

func NewObservation() *pkgregv1.PackageVersionObservationEvent {
	return &pkgregv1.PackageVersionObservationEvent{}
}

func NewMaintainer() *pkgregv1.MaintainerObservationEvent {
	return &pkgregv1.MaintainerObservationEvent{}
}

// Observation is an observation event of tenant. An empty tenant leaves the
// envelope without a tenant.
func Observation(eventID, tenant string) *pkgregv1.PackageVersionObservationEvent {
	return pkgregv1.PackageVersionObservationEvent_builder{Meta: meta(eventID, tenant)}.Build()
}

// Maintainer is a maintainer event of tenant.
func Maintainer(eventID, tenant string) *pkgregv1.MaintainerObservationEvent {
	return pkgregv1.MaintainerObservationEvent_builder{Meta: meta(eventID, tenant)}.Build()
}

// Bytes is the binary proto of m.
func Bytes(t testing.TB, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	require.NoError(t, err)
	return b
}

func meta(eventID, tenant string) *commonv1.EventMeta {
	opts := []events.Option{events.WithEventID(eventID)}
	if tenant != "" {
		opts = append(opts, events.WithTenant(tenant))
	}
	return events.NewMeta(opts...)
}

func randomSuffix(t testing.TB) string {
	t.Helper()
	b := make([]byte, 6)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}

// streamName keeps the letters and digits of the test name, so that an operator
// can tell which test left a stream behind.
func streamName(testName, suffix string) string {
	const maxPrefix = 40
	var b strings.Builder
	for _, c := range testName {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	prefix := b.String()
	if len(prefix) > maxPrefix {
		prefix = prefix[:maxPrefix]
	}
	return prefix + "_" + suffix
}
