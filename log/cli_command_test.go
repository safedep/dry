package log

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunCommand_HappyPath(t *testing.T) {
	var buf bytes.Buffer
	defer SwapGlobalForTest(&buf)()

	err := RunCommand(context.Background(), "backfill", func(ctx context.Context) error {
		Set(ctx, "rows", 10)
		return nil
	})
	assert.NoError(t, err)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "cmd.backfill", got["msg"])
	assert.Equal(t, float64(10), got["rows"])
	assert.Equal(t, "INFO", got["level"])
}

func TestRunCommand_RecordsError(t *testing.T) {
	var buf bytes.Buffer
	defer SwapGlobalForTest(&buf)()

	err := RunCommand(context.Background(), "broken", func(ctx context.Context) error {
		return errors.New("nope")
	})
	assert.Error(t, err)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, "ERROR", got["level"])
	assert.Equal(t, "nope", got["error"])
}
