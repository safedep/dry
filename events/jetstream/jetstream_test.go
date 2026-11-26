package jetstream_test

import (
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	natsjs "github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/internal/natstest"
	"github.com/safedep/dry/events/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const (
	observationFeed = "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"
	maintainerFeed  = "safedep.events.private.packageregistry.v1.MaintainerObservationEvent"
)

func validStream() jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:     "PACKAGEREGISTRY",
		Subjects: []string{"safedep.events.private.packageregistry.v1.>"},
		MaxAge:   time.Hour,
		MaxBytes: 1 << 20,
	}
}

func observation(t *testing.T, eventID string) *pkgregv1.PackageVersionObservationEvent {
	t.Helper()
	return pkgregv1.PackageVersionObservationEvent_builder{
		Meta: events.NewMeta(events.WithEventID(eventID)),
	}.Build()
}

func ensureStream(t *testing.T) (natsjs.JetStream, natsjs.Stream) {
	t.Helper()
	js := natstest.JetStream(t)
	stream, err := jetstream.EnsureStream(t.Context(), js, validStream())
	require.NoError(t, err)
	return js, stream
}

func TestEnsureStream_Validation(t *testing.T) {
	js := natstest.JetStream(t)

	cases := []struct {
		name   string
		mutate func(*jetstream.StreamConfig)
	}{
		{name: "no name", mutate: func(c *jetstream.StreamConfig) { c.Name = "" }},
		{name: "no subjects", mutate: func(c *jetstream.StreamConfig) { c.Subjects = nil }},
		{name: "empty subject", mutate: func(c *jetstream.StreamConfig) { c.Subjects = []string{""} }},
		{name: "no max age", mutate: func(c *jetstream.StreamConfig) { c.MaxAge = 0 }},
		{name: "no max bytes", mutate: func(c *jetstream.StreamConfig) { c.MaxBytes = 0 }},
		{name: "too many replicas", mutate: func(c *jetstream.StreamConfig) { c.Replicas = 6 }},
		{name: "negative replicas", mutate: func(c *jetstream.StreamConfig) { c.Replicas = -1 }},
		{name: "negative duplicate window", mutate: func(c *jetstream.StreamConfig) { c.DuplicateWindow = -time.Second }},
		{name: "duplicate window above max age", mutate: func(c *jetstream.StreamConfig) { c.DuplicateWindow = 2 * time.Hour }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validStream()
			tc.mutate(&cfg)
			_, err := jetstream.EnsureStream(t.Context(), js, cfg)
			assert.Error(t, err)
		})
	}
}

func TestEnsureStream_CreatesThenUpdates(t *testing.T) {
	js := natstest.JetStream(t)

	cfg := validStream()
	cfg.DuplicateWindow = time.Minute
	stream, err := jetstream.EnsureStream(t.Context(), js, cfg)
	require.NoError(t, err)

	info := stream.CachedInfo().Config
	assert.Equal(t, natsjs.FileStorage, info.Storage)
	assert.Equal(t, natsjs.LimitsPolicy, info.Retention)
	assert.Equal(t, natsjs.DiscardOld, info.Discard)
	assert.Equal(t, 1, info.Replicas)
	assert.Equal(t, time.Minute, info.Duplicates)

	cfg.MaxBytes = 2 << 20
	stream, err = jetstream.EnsureStream(t.Context(), js, cfg)
	require.NoError(t, err)
	assert.Equal(t, int64(2<<20), stream.CachedInfo().Config.MaxBytes)
}

func TestEnsureConsumer(t *testing.T) {
	_, stream := ensureStream(t)

	cases := []struct {
		name    string
		cfg     jetstream.ConsumerConfig
		wantErr bool
		check   func(t *testing.T, c natsjs.ConsumerConfig)
	}{
		{name: "whole stream", cfg: jetstream.ConsumerConfig{Durable: "all"},
			check: func(t *testing.T, c natsjs.ConsumerConfig) {
				assert.Empty(t, c.FilterSubject)
				assert.Empty(t, c.FilterSubjects)
			}},
		{name: "one filter", cfg: jetstream.ConsumerConfig{Durable: "one", FilterSubjects: []string{observationFeed + ".*"}},
			check: func(t *testing.T, c natsjs.ConsumerConfig) {
				assert.Equal(t, observationFeed+".*", c.FilterSubject)
			}},
		{name: "two filters", cfg: jetstream.ConsumerConfig{
			Durable:        "two",
			FilterSubjects: []string{observationFeed + ".*", maintainerFeed + ".*"},
			AckWait:        5 * time.Second,
			MaxDeliver:     4,
			MaxAckPending:  50,
		}, check: func(t *testing.T, c natsjs.ConsumerConfig) {
			assert.Equal(t, []string{observationFeed + ".*", maintainerFeed + ".*"}, c.FilterSubjects)
			assert.Equal(t, natsjs.AckExplicitPolicy, c.AckPolicy)
			assert.Equal(t, natsjs.DeliverAllPolicy, c.DeliverPolicy)
			assert.Equal(t, 5*time.Second, c.AckWait)
			assert.Equal(t, 4, c.MaxDeliver)
			assert.Equal(t, 50, c.MaxAckPending)
		}},
		{name: "no durable", cfg: jetstream.ConsumerConfig{}, wantErr: true},
		{name: "empty filter", cfg: jetstream.ConsumerConfig{Durable: "x", FilterSubjects: []string{""}}, wantErr: true},
		{name: "negative ack wait", cfg: jetstream.ConsumerConfig{Durable: "x", AckWait: -time.Second}, wantErr: true},
		{name: "max deliver below -1", cfg: jetstream.ConsumerConfig{Durable: "x", MaxDeliver: -2}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consumer, err := jetstream.EnsureConsumer(t.Context(), stream, tc.cfg)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			info := consumer.CachedInfo().Config
			assert.Equal(t, tc.cfg.Durable, info.Durable)
			tc.check(t, info)
		})
	}
}

func TestPublisher_DedupsOnMessageID(t *testing.T) {
	cases := []struct {
		name   string
		first  []jetstream.PublishOption
		second []jetstream.PublishOption
		ids    [2]string
	}{
		{name: "same event id", ids: [2]string{"evt-1", "evt-1"}},
		{name: "same explicit message id", ids: [2]string{"evt-1", "evt-2"},
			first:  []jetstream.PublishOption{jetstream.WithMsgID("client-1")},
			second: []jetstream.PublishOption{jetstream.WithMsgID("client-1")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			js, stream := ensureStream(t)
			pub, err := jetstream.NewPublisher(js)
			require.NoError(t, err)

			first, err := pub.Publish(t.Context(), observation(t, tc.ids[0]), "tenant-a", tc.first...)
			require.NoError(t, err)
			assert.False(t, first.Duplicate)
			assert.Equal(t, "PACKAGEREGISTRY", first.Stream)

			second, err := pub.Publish(t.Context(), observation(t, tc.ids[1]), "tenant-a", tc.second...)
			require.NoError(t, err, "a duplicate is a success")
			assert.True(t, second.Duplicate)
			assert.Equal(t, first.Sequence, second.Sequence)

			info, err := stream.Info(t.Context())
			require.NoError(t, err)
			assert.Equal(t, uint64(1), info.State.Msgs)

			msg, err := stream.GetMsg(t.Context(), first.Sequence)
			require.NoError(t, err)
			assert.Equal(t, observationFeed+".tenant-a", msg.Subject)
			got := &pkgregv1.PackageVersionObservationEvent{}
			require.NoError(t, proto.Unmarshal(msg.Data, got))
			assert.Equal(t, tc.ids[0], got.GetMeta().GetEventId())
		})
	}
}

func TestPublisher_DistinctIDsAreStored(t *testing.T) {
	js, stream := ensureStream(t)
	pub, err := jetstream.NewPublisher(js)
	require.NoError(t, err)

	for _, id := range []string{"evt-1", "evt-2"} {
		res, err := pub.Publish(t.Context(), observation(t, id), "tenant-a")
		require.NoError(t, err)
		assert.False(t, res.Duplicate)
	}

	info, err := stream.Info(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(2), info.State.Msgs)
}

func publicEvent(t *testing.T) proto.Message {
	t.Helper()
	fd, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:        proto.String("public_test.proto"),
		Package:     proto.String("safedep.events.public.test.v1"),
		Syntax:      proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{Name: proto.String("TestEvent")}},
	}, nil)
	require.NoError(t, err)
	return dynamicpb.NewMessage(fd.Messages().Get(0))
}

func TestPublisher_Refuses(t *testing.T) {
	js, _ := ensureStream(t)
	pub, err := jetstream.NewPublisher(js)
	require.NoError(t, err)

	cases := []struct {
		name   string
		event  proto.Message
		tenant string
	}{
		{name: "public feed", event: publicEvent(t), tenant: "tenant-a"},
		{name: "not an event", event: events.NewMeta(), tenant: "tenant-a"},
		{name: "nil event", event: nil, tenant: "tenant-a"},
		{name: "empty tenant", event: observation(t, "e"), tenant: ""},
		{name: "tenant with a dot", event: observation(t, "e"), tenant: "ab.cd"},
		{name: "wildcard tenant", event: observation(t, "e"), tenant: "*"},
		{name: "no event id", event: pkgregv1.PackageVersionObservationEvent_builder{}.Build(), tenant: "tenant-a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pub.Publish(t.Context(), tc.event, tc.tenant)
			assert.Error(t, err)
		})
	}

	_, err = jetstream.NewPublisher(nil)
	assert.Error(t, err)
}

func TestPurgeTenant_RemovesOnlyThatTenant(t *testing.T) {
	js, stream := ensureStream(t)
	pub, err := jetstream.NewPublisher(js)
	require.NoError(t, err)

	for i, tenant := range []string{"tenant-a", "tenant-b", "tenant-a"} {
		_, err := pub.Publish(t.Context(), observation(t, tenant+"-"+string(rune('0'+i))), tenant)
		require.NoError(t, err)
	}

	routing, err := events.RoutingFor(&pkgregv1.PackageVersionObservationEvent{})
	require.NoError(t, err)
	require.NoError(t, jetstream.PurgeTenant(t.Context(), stream, routing, "tenant-a"))

	info, err := stream.Info(t.Context(), natsjs.WithSubjectFilter(observationFeed+".*"))
	require.NoError(t, err)
	assert.Equal(t, map[string]uint64{observationFeed + ".tenant-b": 1}, info.State.Subjects)

	assert.Error(t, jetstream.PurgeTenant(t.Context(), stream, routing, "*"), "a wildcard would purge every tenant")
}
