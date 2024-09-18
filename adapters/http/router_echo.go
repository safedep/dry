package http

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/labstack/echo-contrib/echoprometheus"
	echo "github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	drylog "github.com/safedep/dry/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

type EchoRouterConfig struct {
	ServiceName         string
	SkipHealthEndpoint  bool
	SkipMetricsEndpoint bool

	// Used to configure prometheus middleware
	MetricsNamespace string
	MetricsSubsystem string
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

	// Copy the request id from response header to the request
	// header so that downstream services can use it. This should
	// come after echo's requestID middleware.
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Request().Header.Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = c.Response().Header().Get(echo.HeaderXRequestID)
				c.Request().Header.Set(echo.HeaderXRequestID, requestID)
			}

			return next(c)
		}
	})

	if !config.SkipHealthEndpoint {
		router.GET(HealthPath, func(c echo.Context) error {
			return c.String(200, "OK")
		})
	}

	if !config.SkipMetricsEndpoint {
		// https://prometheus.io/docs/concepts/data_model/
		metricsNameRegex := regexp.MustCompile("^[a-zA-Z_:][a-zA-Z0-9_:]*$")
		if config.MetricsSubsystem != "" && !metricsNameRegex.MatchString(config.MetricsSubsystem) {
			return nil,
				fmt.Errorf("subsystem name %s is invalid. Must match regex %s", config.MetricsSubsystem,
					metricsNameRegex.String())
		}

		if config.MetricsNamespace != "" && !metricsNameRegex.MatchString(config.MetricsNamespace) {
			return nil,
				fmt.Errorf("namespace name %s is invalid. Must match regex %s", config.MetricsNamespace,
					metricsNameRegex.String())
		}

		router.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
			Subsystem: config.MetricsSubsystem,
			Namespace: config.MetricsNamespace,
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

	// This must be to the end of the middleware chain
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			logger := drylog.With(map[string]interface{}{"request_id": c.Response().Header().Get(echo.HeaderXRequestID)})
			c.Set("dry_logger", logger)

			// TODO: Figure out a way to pass the logger trasparently
			// to the business logic layer. We can also switch to a context logger
			// which flushes the log at the end of the request.
			return next(c)
		}
	})

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
