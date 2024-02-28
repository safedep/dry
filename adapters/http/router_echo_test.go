package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEchoRouterHealthCheck(t *testing.T) {
	router, err := NewEchoRouter(EchoRouterConfig{
		ServiceName: "test",
	})

	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, HealthPath, nil)
	rec := httptest.NewRecorder()

	router.Handler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
