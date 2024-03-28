package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEchoRouterHealthCheck(t *testing.T) {
	router, err := NewEchoRouter(EchoRouterConfig{
		ServiceName:         "test",
		SkipMetricsEndpoint: true,
	})

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, HealthPath, nil)
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsWithInvalidServiceName(t *testing.T) {
	_, err := NewEchoRouter(EchoRouterConfig{
		ServiceName: "test$",
	})

	assert.Error(t, err)
	assert.ErrorContains(t, err, "service name test$ is invalid. Must match regex ^[a-zA-Z_:][a-zA-Z0-9_:]*$")
}

// Need to be careful. Can't register prometheus metrics multiple times.
func TestEchoRouterMetrics(t *testing.T) {
	router, err := NewEchoRouter(EchoRouterConfig{
		ServiceName: "test_sample_service",
	})

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, MetricsPath, nil)
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
