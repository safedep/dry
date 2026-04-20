package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	drylog "github.com/safedep/dry/log"
	"github.com/stretchr/testify/assert"
)

func TestEventLoggingMiddleware_EmitsOneCanonicalLine(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	e := echo.New()
	e.Use(EventLoggingMiddleware())
	e.GET("/hello", func(c echo.Context) error {
		drylog.Set(c.Request().Context(), "user.id", "u1")
		return c.String(http.StatusOK, "hi")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set(echo.HeaderXRequestID, "req-123")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1)

	var got map[string]any
	_ = json.Unmarshal(lines[0], &got)
	assert.Equal(t, "http.request", got["msg"])
	assert.Equal(t, "GET", got["http.method"])
	assert.Equal(t, "/hello", got["http.path"])
	assert.Equal(t, float64(http.StatusOK), got["http.status"])
	assert.Equal(t, "u1", got["user.id"])
	assert.Equal(t, "req-123", got["request_id"])
}

func TestEventLoggingMiddleware_CapturesHandlerPanic(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	e := echo.New()
	e.Use(middleware.Recover()) // converts re-panic to 500
	e.Use(EventLoggingMiddleware())
	e.GET("/boom", func(c echo.Context) error {
		panic("handler exploded")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	assert.Len(t, lines, 1, "one canonical line despite panic")

	var got map[string]any
	_ = json.Unmarshal(lines[0], &got)
	assert.Equal(t, "ERROR", got["level"])
	assert.Equal(t, "handler exploded", got["panic"])
	assert.Contains(t, got, "stack")
	assert.Equal(t, float64(http.StatusInternalServerError), got["http.status"],
		"panic path should report 500")
	assert.Equal(t, "/boom", got["http.route"])
}

func TestEventLoggingMiddleware_DerivesStatusFromHTTPError(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	e := echo.New()
	e.Use(EventLoggingMiddleware())
	e.GET("/teapot", func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "no coffee here")
	})

	req := httptest.NewRequest(http.MethodGet, "/teapot", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTeapot, rec.Code)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, float64(http.StatusTeapot), got["http.status"],
		"canonical line should reflect echo.HTTPError.Code even though Echo's default error handler runs after our middleware")
	assert.Equal(t, "ERROR", got["level"])
}

func TestEventLoggingMiddleware_DerivesStatusFromGenericError(t *testing.T) {
	var buf bytes.Buffer
	defer drylog.SwapGlobalForTest(&buf)()

	e := echo.New()
	e.Use(EventLoggingMiddleware())
	e.GET("/oops", func(c echo.Context) error {
		return assert.AnError
	})

	req := httptest.NewRequest(http.MethodGet, "/oops", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var got map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
	assert.Equal(t, float64(http.StatusInternalServerError), got["http.status"],
		"non-HTTPError should default to 500")
}
