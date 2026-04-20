package async

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	drylog "github.com/safedep/dry/log"
	"github.com/stretchr/testify/assert"
)

func TestWithEventLogging_EmitsCanonicalLine(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	inner := func(ctx context.Context, data []byte, extra MessageExtra) error {
		drylog.Set(ctx, "handler.rows", 42)
		return nil
	}
	wrapped := WithEventLogging("msg.test", inner)

	err := wrapped(context.Background(), []byte("payload"),
		MessageExtra{Subject: "orders.create", ReplyTo: "inbox.1"})
	assert.NoError(t, err)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "msg.test", got["msg"])
	assert.Equal(t, "orders.create", got["msg.subject"])
	assert.Equal(t, "inbox.1", got["msg.reply_to"])
	assert.Equal(t, float64(7), got["msg.bytes"])
	assert.Equal(t, float64(42), got["handler.rows"])
}

func TestWithEventLogging_RecordsErr(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	inner := func(ctx context.Context, data []byte, extra MessageExtra) error {
		return errors.New("bad payload")
	}
	wrapped := WithEventLogging("msg.test", inner)

	err := wrapped(context.Background(), nil, MessageExtra{Subject: "x"})
	assert.Error(t, err)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "ERROR", got["level"])
	assert.Equal(t, "bad payload", got["error"])
}
