package natsjs_test

import (
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/internal/natstest"
	"github.com/safedep/dry/events/natsjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func ensureStream(t *testing.T) (jetstream.JetStream, jetstream.Stream) {
	t.Helper()
	js := natstest.JetStream(t)
	stream, err := natsjs.EnsureStream(t.Context(), js, natstest.StreamConfig())
	require.NoError(t, err)
	return js, stream
}

func TestEnsureStream_Validation(t *testing.T) {
	js := natstest.JetStream(t)

	cases := []struct {
		name   string
		mutate func(*natsjs.StreamConfig)
	}{
		{name: "no name", mutate: func(c *natsjs.StreamConfig) { c.Name = "" }},
		{name: "no subjects", mutate: func(c *natsjs.StreamConfig) { c.Subjects = nil }},
		{name: "empty subject", mutate: func(c *natsjs.StreamConfig) { c.Subjects = []string{""} }},
		{name: "no max age", mutate: func(c *natsjs.StreamConfig) { c.MaxAge = 0 }},
		{name: "no max bytes", mutate: func(c *natsjs.StreamConfig) { c.MaxBytes = 0 }},
		{name: "too many replicas", mutate: func(c *natsjs.StreamConfig) { c.Replicas = 6 }},
		{name: "negative replicas", mutate: func(c *natsjs.StreamConfig) { c.Replicas = -1 }},
		{name: "negative duplicate window", mutate: func(c *natsjs.StreamConfig) { c.DuplicateWindow = -time.Second }},
		{name: "duplicate window above max age", mutate: func(c *natsjs.StreamConfig) { c.DuplicateWindow = 2 * time.Hour }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := natstest.StreamConfig()
			tc.mutate(&cfg)
			_, err := natsjs.EnsureStream(t.Context(), js, cfg)
			assert.Error(t, err)
		})
	}
}

func TestEnsureStream_CreatesAndDetectsDrift(t *testing.T) {
	js := natstest.JetStream(t)

	cfg := natstest.StreamConfig()
	stream, err := natsjs.EnsureStream(t.Context(), js, cfg)
	require.NoError(t, err)
	info := stream.CachedInfo().Config
	assert.Equal(t, jetstream.FileStorage, info.Storage)
	assert.Equal(t, jetstream.LimitsPolicy, info.Retention)
	assert.Equal(t, jetstream.DiscardOld, info.Discard)
	assert.Equal(t, 1, info.Replicas)
	assert.Equal(t, 2*time.Minute, info.Duplicates)

	reordered := cfg
	reordered.Subjects = []string{cfg.Subjects[1], cfg.Subjects[0]}
	_, err = natsjs.EnsureStream(t.Context(), js, reordered)
	require.NoError(t, err, "the same config on the next boot is not a drift")

	lower := cfg
	lower.MaxBytes = 1 << 20
	_, err = natsjs.EnsureStream(t.Context(), js, lower)
	require.ErrorIs(t, err, natsjs.ErrDrift)
	assert.Contains(t, err.Error(), "max bytes")

	current, err := js.Stream(t.Context(), cfg.Name)
	require.NoError(t, err)
	assert.Equal(t, cfg.MaxBytes, current.CachedInfo().Config.MaxBytes, "EnsureStream does not change the stream")

	updated, err := natsjs.UpdateStream(t.Context(), js, lower)
	require.NoError(t, err)
	assert.Equal(t, lower.MaxBytes, updated.CachedInfo().Config.MaxBytes)

	_, err = natsjs.EnsureStream(t.Context(), js, lower)
	assert.NoError(t, err)
}

func TestEnsureConsumer(t *testing.T) {
	_, stream := ensureStream(t)

	cases := []struct {
		name    string
		cfg     natsjs.ConsumerConfig
		wantErr bool
		check   func(t *testing.T, c jetstream.ConsumerConfig)
	}{
		{name: "whole stream with defaults", cfg: natsjs.ConsumerConfig{Durable: "all"},
			check: func(t *testing.T, c jetstream.ConsumerConfig) {
				assert.Empty(t, c.FilterSubject)
				assert.Empty(t, c.FilterSubjects)
				assert.Equal(t, -1, c.MaxDeliver)
				assert.Equal(t, 30*time.Second, c.AckWait)
			}},
		{name: "one filter", cfg: natsjs.ConsumerConfig{Durable: "one", FilterSubjects: []string{natstest.ObservationFeed + ".*"}},
			check: func(t *testing.T, c jetstream.ConsumerConfig) {
				assert.Equal(t, natstest.ObservationFeed+".*", c.FilterSubject)
			}},
		{name: "two filters", cfg: natsjs.ConsumerConfig{
			Durable:        "two",
			FilterSubjects: []string{natstest.ObservationFeed + ".*", natstest.MaintainerFeed + ".*"},
			AckWait:        5 * time.Second,
			MaxAckPending:  50,
		}, check: func(t *testing.T, c jetstream.ConsumerConfig) {
			assert.Equal(t, []string{natstest.ObservationFeed + ".*", natstest.MaintainerFeed + ".*"}, c.FilterSubjects)
			assert.Equal(t, jetstream.AckExplicitPolicy, c.AckPolicy)
			assert.Equal(t, jetstream.DeliverAllPolicy, c.DeliverPolicy)
			assert.Equal(t, 5*time.Second, c.AckWait)
			assert.Equal(t, 50, c.MaxAckPending)
		}},
		{name: "no durable", cfg: natsjs.ConsumerConfig{}, wantErr: true},
		{name: "empty filter", cfg: natsjs.ConsumerConfig{Durable: "x", FilterSubjects: []string{""}}, wantErr: true},
		{name: "negative ack wait", cfg: natsjs.ConsumerConfig{Durable: "x", AckWait: -time.Second}, wantErr: true},
		{name: "max deliver below -1", cfg: natsjs.ConsumerConfig{Durable: "x", MaxDeliver: -2}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			consumer, err := natsjs.EnsureConsumer(t.Context(), stream, tc.cfg)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			info := consumer.CachedInfo().Config
			assert.Equal(t, tc.cfg.Durable, info.Durable)
			tc.check(t, info)

			_, err = natsjs.EnsureConsumer(t.Context(), stream, tc.cfg)
			assert.NoError(t, err, "the same config on the next boot is not a drift")
		})
	}
}

func TestEnsureConsumer_DetectsDrift(t *testing.T) {
	_, stream := ensureStream(t)

	cfg := natsjs.ConsumerConfig{Durable: "writer", AckWait: 10 * time.Second}
	_, err := natsjs.EnsureConsumer(t.Context(), stream, cfg)
	require.NoError(t, err)

	changed := cfg
	changed.AckWait = 20 * time.Second
	changed.FilterSubjects = []string{natstest.ObservationFeed + ".*"}
	_, err = natsjs.EnsureConsumer(t.Context(), stream, changed)
	require.ErrorIs(t, err, natsjs.ErrDrift)
	assert.Contains(t, err.Error(), "ack wait")
	assert.Contains(t, err.Error(), "filter subjects")

	current, err := stream.Consumer(t.Context(), cfg.Durable)
	require.NoError(t, err)
	assert.Equal(t, cfg.AckWait, current.CachedInfo().Config.AckWait, "EnsureConsumer does not change the consumer")

	updated, err := natsjs.UpdateConsumer(t.Context(), stream, changed)
	require.NoError(t, err)
	assert.Equal(t, changed.AckWait, updated.CachedInfo().Config.AckWait)

	_, err = natsjs.EnsureConsumer(t.Context(), stream, changed)
	assert.NoError(t, err)
}

func TestPublisher_DedupsOnMessageID(t *testing.T) {
	cases := []struct {
		name   string
		first  []natsjs.PublishOption
		second []natsjs.PublishOption
		ids    [2]string
	}{
		{name: "same event id", ids: [2]string{"evt-1", "evt-1"}},
		{name: "same explicit message id", ids: [2]string{"evt-1", "evt-2"},
			first:  []natsjs.PublishOption{natsjs.WithMsgID("client-1")},
			second: []natsjs.PublishOption{natsjs.WithMsgID("client-1")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			js, stream := ensureStream(t)
			pub, err := natsjs.NewPublisher(js)
			require.NoError(t, err)

			first, err := pub.Publish(t.Context(), natstest.Observation(tc.ids[0], natstest.Tenant), tc.first...)
			require.NoError(t, err)
			assert.False(t, first.Duplicate)
			assert.Equal(t, natstest.StreamName, first.Stream)

			second, err := pub.Publish(t.Context(), natstest.Observation(tc.ids[1], natstest.Tenant), tc.second...)
			require.NoError(t, err, "a duplicate is a success")
			assert.True(t, second.Duplicate)
			assert.Equal(t, first.Sequence, second.Sequence)

			info, err := stream.Info(t.Context())
			require.NoError(t, err)
			assert.Equal(t, uint64(1), info.State.Msgs)

			msg, err := stream.GetMsg(t.Context(), first.Sequence)
			require.NoError(t, err)
			assert.Equal(t, natstest.ObservationFeed+"."+natstest.Tenant, msg.Subject, "the tenant comes from the envelope")
			got := &pkgregv1.PackageVersionObservationEvent{}
			require.NoError(t, proto.Unmarshal(msg.Data, got))
			assert.Equal(t, tc.ids[0], got.GetMeta().GetEventId())
		})
	}
}

func TestPublisher_DistinctIDsAreStored(t *testing.T) {
	js, stream := ensureStream(t)
	pub, err := natsjs.NewPublisher(js)
	require.NoError(t, err)

	for _, id := range []string{"evt-1", "evt-2"} {
		res, err := pub.Publish(t.Context(), natstest.Observation(id, natstest.Tenant))
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
	pub, err := natsjs.NewPublisher(js)
	require.NoError(t, err)

	cases := []struct {
		name  string
		event proto.Message
	}{
		{name: "public feed", event: publicEvent(t)},
		{name: "not an event", event: events.NewMeta()},
		{name: "nil event", event: nil},
		{name: "envelope without a tenant", event: natstest.Observation("e", "")},
		{name: "tenant domain", event: natstest.Observation("e", "ab.cd.safedep.io")},
		{name: "wildcard tenant", event: natstest.Observation("e", "*")},
		{name: "no event id", event: pkgregv1.PackageVersionObservationEvent_builder{
			Meta: events.NewMeta(events.WithEventID(""), events.WithTenant(natstest.Tenant)),
		}.Build()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pub.Publish(t.Context(), tc.event)
			assert.Error(t, err)
		})
	}

	_, err = natsjs.NewPublisher(nil)
	assert.Error(t, err)
}

func TestPurgeTenant_RemovesOnlyThatTenant(t *testing.T) {
	js, stream := ensureStream(t)
	pub, err := natsjs.NewPublisher(js)
	require.NoError(t, err)

	for _, ev := range []proto.Message{
		natstest.Observation("a-1", "tenant-a"),
		natstest.Observation("b-1", "tenant-b"),
		natstest.Observation("a-2", "tenant-a"),
	} {
		_, err := pub.Publish(t.Context(), ev)
		require.NoError(t, err)
	}

	routing, err := events.RoutingFor(natstest.NewObservation())
	require.NoError(t, err)
	require.NoError(t, natsjs.PurgeTenant(t.Context(), stream, routing, "tenant-a"))

	info, err := stream.Info(t.Context(), jetstream.WithSubjectFilter(natstest.ObservationFeed+".*"))
	require.NoError(t, err)
	assert.Equal(t, map[string]uint64{natstest.ObservationFeed + ".tenant-b": 1}, info.State.Subjects)

	assert.Error(t, natsjs.PurgeTenant(t.Context(), stream, routing, "*"), "a wildcard would purge every tenant")
}
