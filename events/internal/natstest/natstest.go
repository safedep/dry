// Package natstest runs an in-process NATS server with JetStream for tests.
package natstest

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/require"
)

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
