package obs

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func WrapHttpHandler(handler http.Handler, operation string,
	options ...otelhttp.Option) http.Handler {
	options = append(options, otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents))
	return otelhttp.NewHandler(handler, operation, options...)
}
