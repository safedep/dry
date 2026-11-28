package natsjs_test

import (
	"fmt"
	"testing"
	"time"

	pkgregv1 "buf.build/gen/go/safedep/api/protocolbuffers/go/safedep/events/private/packageregistry/v1"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/safedep/dry/async"
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

func ensureStream(t *testing.T) (natstest.Server, jetstream.Stream) {
	t.Helper()
	srv := natstest.Connect(t)
	stream, err := natsjs.EnsureStream(t.Context(), srv.JS, srv.StreamConfig())
	require.NoError(t, err)
	return srv, stream
}

func TestEnsureStream_Validation(t *testing.T) {
	srv := natstest.Connect(t)

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
		{name: "unknown discard policy", mutate: func(c *natsjs.StreamConfig) { c.Discard = 7 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := srv.StreamConfig()
			tc.mutate(&cfg)
			_, err := natsjs.EnsureStream(t.Context(), srv.JS, cfg)
			assert.Error(t, err)
		})
	}
}

func TestEnsureStream_CreatesAndDetectsDrift(t *testing.T) {
	srv := natstest.Connect(t)
	js := srv.JS

	cfg := srv.StreamConfig()
	stream, err := natsjs.EnsureStream(t.Context(), js, cfg)
	require.NoError(t, err)
	info := stream.CachedInfo().Config
	assert.Equal(t, jetstream.FileStorage, info.Storage)
	assert.Equal(t, jetstream.LimitsPolicy, info.Retention)
	assert.Equal(t, jetstream.DiscardNew, info.Discard)
	assert.Equal(t, 1, info.Replicas)
	assert.Equal(t, 2*time.Minute, info.Duplicates)

	reordered := cfg
	reordered.Subjects = []string{cfg.Subjects[1], cfg.Subjects[0]}
	_, err = natsjs.EnsureStream(t.Context(), js, reordered)
	require.NoError(t, err, "the same config on the next boot is not a drift")

	lower := cfg
	lower.MaxBytes = 1 << 20
	drifted, err := natsjs.EnsureStream(t.Context(), js, lower)
	require.ErrorIs(t, err, natsjs.ErrDrift)
	assert.Contains(t, err.Error(), "max bytes")
	require.NotNil(t, drifted, "the caller gets the existing stream")
	assert.Equal(t, cfg.MaxBytes, drifted.CachedInfo().Config.MaxBytes)

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
	srv, stream := ensureStream(t)
	observations := natstest.ObservationFeed + "." + srv.Segment
	maintainers := natstest.MaintainerFeed + "." + srv.Segment

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
		{name: "one filter", cfg: natsjs.ConsumerConfig{Durable: "one", FilterSubjects: []string{observations}},
			check: func(t *testing.T, c jetstream.ConsumerConfig) {
				assert.Equal(t, observations, c.FilterSubject)
			}},
		{name: "two filters", cfg: natsjs.ConsumerConfig{
			Durable:        "two",
			FilterSubjects: []string{observations, maintainers},
			AckWait:        5 * time.Second,
			MaxAckPending:  50,
		}, check: func(t *testing.T, c jetstream.ConsumerConfig) {
			assert.Equal(t, []string{observations, maintainers}, c.FilterSubjects)
			assert.Equal(t, jetstream.AckExplicitPolicy, c.AckPolicy)
			assert.Equal(t, jetstream.DeliverAllPolicy, c.DeliverPolicy)
			assert.Equal(t, 5*time.Second, c.AckWait)
			assert.Equal(t, 50, c.MaxAckPending)
		}},
		{name: "no durable", cfg: natsjs.ConsumerConfig{}, wantErr: true},
		{name: "empty filter", cfg: natsjs.ConsumerConfig{Durable: "x", FilterSubjects: []string{""}}, wantErr: true},
		{name: "negative ack wait", cfg: natsjs.ConsumerConfig{Durable: "x", AckWait: -time.Second}, wantErr: true},
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
	srv, stream := ensureStream(t)

	cfg := natsjs.ConsumerConfig{Durable: "writer", AckWait: 10 * time.Second}
	_, err := natsjs.EnsureConsumer(t.Context(), stream, cfg)
	require.NoError(t, err)

	changed := cfg
	changed.AckWait = 20 * time.Second
	changed.FilterSubjects = []string{natstest.ObservationFeed + "." + srv.Segment}
	drifted, err := natsjs.EnsureConsumer(t.Context(), stream, changed)
	require.ErrorIs(t, err, natsjs.ErrDrift)
	assert.Contains(t, err.Error(), "ack wait")
	require.NotNil(t, drifted, "the caller gets the existing consumer")
	assert.Equal(t, cfg.AckWait, drifted.CachedInfo().Config.AckWait)
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

func TestEnsureConsumer_DetectsDriftOfFieldsItDoesNotExpose(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*jetstream.ConsumerConfig)
		want   string
	}{
		{name: "delivery limit", mutate: func(c *jetstream.ConsumerConfig) { c.MaxDeliver = 10 }, want: "max deliver is 10"},
		{name: "ack all", mutate: func(c *jetstream.ConsumerConfig) { c.AckPolicy = jetstream.AckAllPolicy }, want: "ack policy"},
		{name: "headers only", mutate: func(c *jetstream.ConsumerConfig) { c.HeadersOnly = true }, want: "headers only"},
		{name: "request batch limit", mutate: func(c *jetstream.ConsumerConfig) { c.MaxRequestBatch = 10 }, want: "max request batch is 10"},
		{name: "request wait limit", mutate: func(c *jetstream.ConsumerConfig) { c.MaxRequestExpires = time.Second }, want: "max request expires"},
		{name: "request size limit", mutate: func(c *jetstream.ConsumerConfig) { c.MaxRequestMaxBytes = 1024 }, want: "max request max bytes"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stream := ensureStream(t)

			// The raw API makes the consumer, as an operator could.
			cfg := jetstream.ConsumerConfig{
				Durable:       "writer",
				AckPolicy:     jetstream.AckExplicitPolicy,
				AckWait:       30 * time.Second,
				MaxAckPending: 1000,
				MaxDeliver:    -1,
			}
			tc.mutate(&cfg)
			_, err := stream.CreateConsumer(t.Context(), cfg)
			require.NoError(t, err)

			_, err = natsjs.EnsureConsumer(t.Context(), stream, natsjs.ConsumerConfig{Durable: "writer"})
			require.ErrorIs(t, err, natsjs.ErrDrift)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestEnsureStream_DetectsDriftOfFieldsItDoesNotExpose(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*jetstream.StreamConfig)
		want   string
	}{
		{name: "message limit", mutate: func(c *jetstream.StreamConfig) { c.MaxMsgs = 100 }, want: "max msgs is 100"},
		{name: "message limit per subject", mutate: func(c *jetstream.StreamConfig) { c.MaxMsgsPerSubject = 10 }, want: "max msgs per subject"},
		{name: "message size limit", mutate: func(c *jetstream.StreamConfig) { c.MaxMsgSize = 1024 }, want: "max msg size"},
		{name: "discard old", mutate: func(c *jetstream.StreamConfig) { c.Discard = jetstream.DiscardOld }, want: "discard"},
		{name: "work queue", mutate: func(c *jetstream.StreamConfig) { c.Retention = jetstream.WorkQueuePolicy }, want: "retention"},
		{name: "no publish ack", mutate: func(c *jetstream.StreamConfig) { c.NoAck = true }, want: "no ack"},
		{
			name: "subject transform",
			mutate: func(c *jetstream.StreamConfig) {
				c.SubjectTransform = &jetstream.SubjectTransformConfig{Source: c.Subjects[0], Destination: c.Subjects[1]}
			},
			want: "subject transform",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := natstest.Connect(t)
			want := srv.StreamConfig()

			// The raw API makes the stream, as an operator could.
			cfg := jetstream.StreamConfig{
				Name:              want.Name,
				Subjects:          want.Subjects,
				Storage:           jetstream.FileStorage,
				Retention:         jetstream.LimitsPolicy,
				Discard:           jetstream.DiscardNew,
				MaxAge:            want.MaxAge,
				MaxBytes:          want.MaxBytes,
				MaxMsgs:           -1,
				MaxMsgsPerSubject: -1,
				MaxMsgSize:        -1,
				Replicas:          1,
				Duplicates:        2 * time.Minute,
			}
			tc.mutate(&cfg)
			_, err := srv.JS.CreateStream(t.Context(), cfg)
			require.NoError(t, err)

			_, err = natsjs.EnsureStream(t.Context(), srv.JS, want)
			require.ErrorIs(t, err, natsjs.ErrDrift)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
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
			srv, stream := ensureStream(t)
			pub, err := natsjs.NewPublisher(srv.JS)
			require.NoError(t, err)

			first, err := pub.Publish(t.Context(), natstest.Observation(tc.ids[0], srv.Segment), tc.first...)
			require.NoError(t, err)
			assert.False(t, first.Duplicate)
			assert.Equal(t, srv.Stream, first.Stream)

			second, err := pub.Publish(t.Context(), natstest.Observation(tc.ids[1], srv.Segment), tc.second...)
			require.NoError(t, err, "a duplicate is a success")
			assert.True(t, second.Duplicate)
			assert.Equal(t, first.Sequence, second.Sequence)

			info, err := stream.Info(t.Context())
			require.NoError(t, err)
			assert.Equal(t, uint64(1), info.State.Msgs)

			msg, err := stream.GetMsg(t.Context(), first.Sequence)
			require.NoError(t, err)
			assert.Equal(t, natstest.ObservationFeed+"."+srv.Segment, msg.Subject, "the tenant comes from the envelope")
			got := &pkgregv1.PackageVersionObservationEvent{}
			require.NoError(t, proto.Unmarshal(msg.Data, got))
			assert.Equal(t, tc.ids[0], got.GetMeta().GetEventId())
		})
	}
}

func TestPublisher_FullStream(t *testing.T) {
	cases := []struct {
		name     string
		discard  natsjs.DiscardPolicy
		wantFail bool
	}{
		{name: "discard new refuses the publish", discard: natsjs.DiscardNew, wantFail: true},
		{name: "discard old drops the oldest record", discard: natsjs.DiscardOld},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := natstest.Connect(t)
			cfg := srv.StreamConfig()
			cfg.MaxBytes = 4 << 10
			cfg.Discard = tc.discard
			stream, err := natsjs.EnsureStream(t.Context(), srv.JS, cfg)
			require.NoError(t, err)
			pub, err := natsjs.NewPublisher(srv.JS)
			require.NoError(t, err)

			var failed int
			for i := range 200 {
				if _, err := pub.Publish(t.Context(), natstest.Observation(fmt.Sprintf("evt-%d", i), srv.Segment)); err != nil {
					failed++
				}
			}
			info, err := stream.Info(t.Context())
			require.NoError(t, err)
			if tc.wantFail {
				assert.Positive(t, failed, "a full stream refuses the publish")
				assert.Equal(t, uint64(1), info.State.FirstSeq, "no stored record is lost")
				return
			}
			assert.Zero(t, failed)
			assert.Greater(t, info.State.FirstSeq, uint64(1))
		})
	}
}

func TestPublisher_OneEventIDOfTwoTenantsOrFeedsGivesTwoRecords(t *testing.T) {
	srv := natstest.Connect(t)
	other := srv.Segment + "-b"
	stream, err := natsjs.EnsureStream(t.Context(), srv.JS, srv.StreamConfig(srv.Segment, other))
	require.NoError(t, err)
	pub, err := natsjs.NewPublisher(srv.JS)
	require.NoError(t, err)

	for _, ev := range []proto.Message{
		natstest.Observation("evt-1", srv.Segment),
		natstest.Observation("evt-1", other),
		natstest.Maintainer("evt-1", srv.Segment),
	} {
		res, err := pub.Publish(t.Context(), ev)
		require.NoError(t, err)
		assert.False(t, res.Duplicate)
	}

	info, err := stream.Info(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(3), info.State.Msgs)
}

func TestPublisher_DistinctIDsAreStored(t *testing.T) {
	srv, stream := ensureStream(t)
	pub, err := natsjs.NewPublisher(srv.JS)
	require.NoError(t, err)

	for _, id := range []string{"evt-1", "evt-2"} {
		res, err := pub.Publish(t.Context(), natstest.Observation(id, srv.Segment))
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
	srv, _ := ensureStream(t)
	pub, err := natsjs.NewPublisher(srv.JS)
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
			Meta: events.NewMeta(events.WithEventID(""), events.WithTenant(srv.Segment)),
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
	srv := natstest.Connect(t)
	other := srv.Segment + "-b"
	stream, err := natsjs.EnsureStream(t.Context(), srv.JS, srv.StreamConfig(srv.Segment, other))
	require.NoError(t, err)
	pub, err := natsjs.NewPublisher(srv.JS)
	require.NoError(t, err)

	for _, ev := range []proto.Message{
		natstest.Observation("a-1", srv.Segment),
		natstest.Observation("b-1", other),
		natstest.Observation("a-2", srv.Segment),
	} {
		_, err := pub.Publish(t.Context(), ev)
		require.NoError(t, err)
	}

	routing, err := events.RoutingFor(natstest.NewObservation())
	require.NoError(t, err)
	segment, err := async.NewTenantStreamSegmentID(srv.Segment)
	require.NoError(t, err)
	require.NoError(t, natsjs.PurgeTenant(t.Context(), stream, routing, segment))

	info, err := stream.Info(t.Context())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), info.State.Msgs)
	_, err = stream.GetLastMsgForSubject(t.Context(), natstest.ObservationFeed+"."+other)
	require.NoError(t, err, "the other tenant keeps its record")
	_, err = stream.GetLastMsgForSubject(t.Context(), natstest.ObservationFeed+"."+srv.Segment)
	assert.ErrorIs(t, err, jetstream.ErrMsgNotFound)

	assert.Error(t, natsjs.PurgeTenant(t.Context(), stream, routing, async.TenantStreamSegmentID{}),
		"an empty segment id is refused")
}
