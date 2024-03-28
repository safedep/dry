package http

import (
	"net/http"
)

type RouteMethod string

const (
	GET     RouteMethod = "GET"
	POST    RouteMethod = "POST"
	PUT     RouteMethod = "PUT"
	DELETE  RouteMethod = "DELETE"
	OPTIONS RouteMethod = "OPTIONS"
	ANY     RouteMethod = "ANY"

	HealthPath  = "/health"
	MetricsPath = "/metrics"
)

type Router interface {
	AddRoute(method RouteMethod, path string, handler http.Handler)
	Handler() http.Handler
	ListenAndServe(address string) error
}
