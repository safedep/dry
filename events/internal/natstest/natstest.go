// Package natstest holds test fixtures for the JetStream transport: an
// in-process NATS server, two private feeds and their events.
package natstest

import (
	"testing"
	"time"

	commonv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/common/v1"
	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/natsjs"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const (
	ObservationFeed = "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"
	MaintainerFeed  = "safedep.events.private.packageregistry.v1.MaintainerObservationEvent"
	Tenant          = "tenant-a"
	StreamName      = "PACKAGEREGISTRY"
)

// StreamConfig is a stream over both feeds.
func StreamConfig() natsjs.StreamConfig {
	return natsjs.StreamConfig{
		Name:     StreamName,
		Subjects: []string{ObservationFeed + ".*", MaintainerFeed + ".*"},
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

// JetStream starts a server with a store in a temporary directory and returns a
// JetStream context on a new connection. The test cleanup stops both.
func JetStream(t testing.TB) jetstream.JetStream {
	t.Helper()

	srv, err := server.NewServer(&server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
		NoLog:     true,
		NoSigs:    true,
	})
	require.NoError(t, err)

	go srv.Start()
	t.Cleanup(func() {
		srv.Shutdown()
		srv.WaitForShutdown()
	})
	require.True(t, srv.ReadyForConnections(10*time.Second), "NATS server did not start")

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	js, err := jetstream.New(nc)
	require.NoError(t, err)

	return js
}
