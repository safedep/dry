package http

import (
	"net/http"

	"github.com/labstack/echo-contrib/echoprometheus"
	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

type EchoRouterConfig struct {
	ServiceName         string
	SkipHealthEndpoint  bool
	SkipMetricsEndpoint bool
}

type EchoRouter struct {
	config EchoRouterConfig
	router *echo.Echo
}

func NewEchoRouter(config EchoRouterConfig) (Router, error) {
	router := echo.New()
	router.Logger.SetLevel(log.INFO)

	router.Use(middleware.Logger())
	router.Use(middleware.Recover())
	router.Use(middleware.RequestID())
	router.Use(otelecho.Middleware(config.ServiceName))

	if !config.SkipHealthEndpoint {
		router.GET(HealthPath, func(c echo.Context) error {
			return c.String(200, "OK")
		})
	}

	if !config.SkipMetricsEndpoint {
		router.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
			Subsystem: config.ServiceName,
			Skipper: func(c echo.Context) bool {
				if c.Path() == MetricsPath {
					return true
				}

				if c.Path() == HealthPath {
					return true
				}

				return false
			},
		}))

		router.GET(MetricsPath, echoprometheus.NewHandler())
	}

	return &EchoRouter{
		config: config,
		router: router,
	}, nil
}

func (r *EchoRouter) AddRoute(method RouteMethod, path string, handler http.Handler) {
	switch method {
	case GET:
		r.router.GET(path, echo.WrapHandler(handler))
	case POST:
		r.router.POST(path, echo.WrapHandler(handler))
	case PUT:
		r.router.PUT(path, echo.WrapHandler(handler))
	case DELETE:
		r.router.DELETE(path, echo.WrapHandler(handler))
	case OPTIONS:
		r.router.OPTIONS(path, echo.WrapHandler(handler))
	case ANY:
		r.router.Any(path, echo.WrapHandler(handler))
	}
}

func (r *EchoRouter) Handler() http.Handler {
	return r.router
}

func (r *EchoRouter) ListenAndServe(address string) error {
	return r.router.Start(address)
}
