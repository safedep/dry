package destinations

import (
	"context"
	"testing"

	"github.com/safedep/dry/events"
	"github.com/safedep/dry/events/outbox"
	"github.com/safedep/dry/stream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatsDestination_Accepts(t *testing.T) {
	d := NewNATS(nil)

	public := events.Routing{Exposure: events.ExposurePublic, Domain: "threatintel", Major: 1, Message: "VerdictsEvent"}
	private := events.Routing{Exposure: events.ExposurePrivate, Domain: "packageregistry", Major: 1, Message: "PackageVersionObservationEvent"}

	assert.False(t, d.Accepts(public), "NATS must reject public feeds")
	assert.True(t, d.Accepts(private), "NATS accepts private feeds")
}

func TestS2Destination_Accepts(t *testing.T) {
	d := NewS2(stream.S2StreamProviderConfig{ApiKey: "test"}, nil)

	assert.True(t, d.Accepts(events.Routing{Exposure: events.ExposurePublic}))
	assert.True(t, d.Accepts(events.Routing{Exposure: events.ExposurePrivate}))
}

type plainPublisher struct {
	topic string
	data  []byte
}

func (p *plainPublisher) Publish(_ context.Context, topic string, data []byte) error {
	p.topic, p.data = topic, data
	return nil
}

type headerPublisher struct {
	plainPublisher
	headers map[string][]string
}

func (p *headerPublisher) PublishWithHeaders(_ context.Context, topic string, data []byte, headers map[string][]string) error {
	p.topic, p.data, p.headers = topic, data, headers
	return nil
}

func TestNatsDestination_PublishSetsMsgIDWhenPublisherHasHeaders(t *testing.T) {
	routing := events.Routing{Exposure: events.ExposurePrivate, Domain: "packageregistry", Major: 1, Message: "PackageVersionObservationEvent"}
	subject := "safedep.events.private.packageregistry.v1.PackageVersionObservationEvent"

	cases := []struct {
		name        string
		eventID     string
		withHeaders bool
		wantHeaders map[string][]string
	}{
		{name: "header publisher gets the event id", eventID: "evt-1", withHeaders: true, wantHeaders: map[string][]string{"Nats-Msg-Id": {"evt-1"}}},
		{name: "header publisher without an event id", eventID: "", withHeaders: true},
		{name: "plain publisher", eventID: "evt-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := outbox.PublishRequest{Routing: routing, EventID: tc.eventID, Record: []byte("rec")}
			if tc.withHeaders {
				pub := &headerPublisher{}
				require.NoError(t, NewNATS(pub).Publish(t.Context(), req))
				assert.Equal(t, subject, pub.topic)
				assert.Equal(t, []byte("rec"), pub.data)
				assert.Equal(t, tc.wantHeaders, pub.headers)
				return
			}
			pub := &plainPublisher{}
			require.NoError(t, NewNATS(pub).Publish(t.Context(), req))
			assert.Equal(t, subject, pub.topic)
			assert.Equal(t, []byte("rec"), pub.data)
		})
	}
}
