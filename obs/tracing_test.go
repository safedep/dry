package obs

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/trace"
)

func tracerTestSetEnv() {
	os.Setenv("APP_SERVICE_OBS_ENABLED", "true")
	os.Setenv("APP_SERVICE_NAME", "obs-testing")
	os.Setenv("APP_SERVICE_ENV", "development")
	os.Setenv("APP_OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:8888")
}

func tracerSetUnsetEnv() {
	os.Setenv("APP_SERVICE_OBS_ENABLED", "")
	os.Setenv("APP_SERVICE_NAME", "")
	os.Setenv("APP_SERVICE_LABELS", "")
	os.Setenv("APP_OTEL_EXPORTER_OTLP_ENDPOINT", "")
}

func TestSpannedSetStatusOkOnSuccess(t *testing.T) {
	tracerTestSetEnv()
	t.Cleanup(tracerSetUnsetEnv)

	shutdown := InitTracing()
	defer shutdown(context.Background())

	var span trace.Span
	err := Spanned(context.Background(), "span.test", func(ctx context.Context) error {
		span = trace.SpanFromContext(ctx)
		return nil
	})

	assert.Nil(t, err)
	assert.False(t, span.IsRecording())
	assert.True(t, span.SpanContext().IsValid())
}
