package stream

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/s2-streamstore/s2-sdk-go/s2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeS2Position_RoundTrips(t *testing.T) {
	tests := []struct {
		name string
		seq  uint64
	}{
		{"zero", 0},
		{"one", 1},
		{"large", 18446744073709551615}, // max uint64
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeS2Position(tt.seq)
			decoded, err := decodeS2Position(encoded)
			require.NoError(t, err)
			assert.Equal(t, tt.seq, decoded)
		})
	}
}

func TestDecodeS2Position(t *testing.T) {
	t.Run("empty is the beginning", func(t *testing.T) {
		seq, err := decodeS2Position("")
		require.NoError(t, err)
		assert.Equal(t, uint64(0), seq)
	})
	t.Run("non-numeric is rejected", func(t *testing.T) {
		_, err := decodeS2Position("not-a-number")
		require.Error(t, err)
	})
}

func TestS2ReadOptionsFrom(t *testing.T) {
	t.Run("start position wins over from-tail", func(t *testing.T) {
		got, err := s2ReadOptionsFrom(StreamReadOptions{StartPosition: "42", FromTail: true})
		require.NoError(t, err)
		require.NotNil(t, got.SeqNum)
		assert.Equal(t, uint64(42), *got.SeqNum)
		assert.Nil(t, got.TailOffset)
		assert.True(t, got.IgnoreCommandRecords)
	})
	t.Run("from tail when no position", func(t *testing.T) {
		got, err := s2ReadOptionsFrom(StreamReadOptions{FromTail: true})
		require.NoError(t, err)
		require.NotNil(t, got.TailOffset)
		assert.Equal(t, int64(0), *got.TailOffset)
		assert.Nil(t, got.SeqNum)
		assert.True(t, got.IgnoreCommandRecords)
	})
	t.Run("empty options read from the beginning", func(t *testing.T) {
		got, err := s2ReadOptionsFrom(StreamReadOptions{})
		require.NoError(t, err)
		require.NotNil(t, got.SeqNum)
		assert.Equal(t, uint64(0), *got.SeqNum)
	})
	t.Run("invalid start position is rejected", func(t *testing.T) {
		_, err := s2ReadOptionsFrom(StreamReadOptions{StartPosition: "xyz"})
		require.Error(t, err)
	})
}

func TestStreamRecordFromS2(t *testing.T) {
	rec := s2.SequencedRecord{
		Body:    []byte("payload"),
		SeqNum:  7,
		Headers: []s2.Header{s2.NewHeader("k1", "v1"), s2.NewHeader("k2", "v2")},
	}

	got := streamRecordFromS2(rec)

	assert.Equal(t, []byte("payload"), got.Body)
	assert.Equal(t, map[string]string{"k1": "v1", "k2": "v2"}, got.Headers)
	// Position is this record; Next is one past it, so a consumer that persists
	// Next resumes at SeqNum+1 rather than replaying SeqNum.
	assert.Equal(t, "7", got.Position)
	assert.Equal(t, "8", got.Next)
}

func TestStreamRecordFromS2_NoHeaders(t *testing.T) {
	got := streamRecordFromS2(s2.SequencedRecord{Body: []byte("x"), SeqNum: 0})
	assert.Empty(t, got.Headers)
	assert.Equal(t, "0", got.Position)
	assert.Equal(t, "1", got.Next)
}

func TestStreamRecordFromS2_NextDoesNotWrapAtMax(t *testing.T) {
	got := streamRecordFromS2(s2.SequencedRecord{Body: []byte("x"), SeqNum: math.MaxUint64})
	// At the max sequence number Next stays pinned rather than wrapping to 0,
	// which would rewind the cursor to the start of the stream.
	assert.Equal(t, encodeS2Position(math.MaxUint64), got.Next)
	assert.NotEqual(t, "0", got.Next)
}

func TestResolveTerminalErr(t *testing.T) {
	sessErr := errors.New("transport boom")

	t.Run("session error wins", func(t *testing.T) {
		_, err := resolveTerminalErr(sessErr, context.Canceled)
		assert.ErrorIs(t, err, sessErr)
	})
	t.Run("falls back to context error when session error is nil", func(t *testing.T) {
		// The S2 SDK can close its channels on cancellation without setting Err();
		// the bound context's error must surface instead of a clean io.EOF.
		_, err := resolveTerminalErr(nil, context.Canceled)
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("clean end is io.EOF", func(t *testing.T) {
		_, err := resolveTerminalErr(nil, nil)
		assert.ErrorIs(t, err, io.EOF)
	})
}

func TestNewS2StreamReadSession_RequiresApiKey(t *testing.T) {
	_, err := NewS2StreamReadSession(t.Context(),
		S2StreamProviderConfig{ApiKey: ""},
		NewDefaultS2BasinResolver(),
		Stream{Namespace: "ns", Name: "feed"},
		StreamReadOptions{})
	require.Error(t, err)
}
