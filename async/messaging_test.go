package async

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPublishWithHeaders(t *testing.T) {
	mockService := NewMockMessagingService(t)

	topic := "test.topic"
	data := []byte("test-data")
	headers := map[string][]string{
		"X-Request-Id": {"req-123"},
		"X-Trace-Id":   {"trace-456", "trace-789"},
	}

	mockService.EXPECT().
		PublishWithHeaders(mock.Anything, topic, data, headers).
		Return(nil)

	err := mockService.PublishWithHeaders(context.Background(), topic, data, headers)
	assert.NoError(t, err)
}

func TestPublishWithHeadersError(t *testing.T) {
	mockService := NewMockMessagingService(t)

	topic := "test.topic"
	data := []byte("test-data")
	headers := map[string][]string{
		"X-Request-Id": {"req-123"},
	}

	mockService.EXPECT().
		PublishWithHeaders(mock.Anything, topic, data, headers).
		Return(assert.AnError)

	err := mockService.PublishWithHeaders(context.Background(), topic, data, headers)
	assert.ErrorIs(t, err, assert.AnError)
}
