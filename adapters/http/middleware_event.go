package http

import (
	echo "github.com/labstack/echo/v4"
	drylog "github.com/safedep/dry/log"
)

// EventLoggingMiddleware starts a canonical logging event for each
// request and emits one JSON line at the end. Must run AFTER
// middleware.Recover() (so panics are converted to 500 after our event
// is flushed) and AFTER middleware.RequestID() (so the ID is set).
func EventLoggingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			requestID := req.Header.Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = c.Response().Header().Get(echo.HeaderXRequestID)
			}

			ctx, end := drylog.BeginEvent(req.Context(), "http.request",
				drylog.WithEventAttrs(map[string]any{
					"request_id":  requestID,
					"http.method": req.Method,
					"http.path":   req.URL.Path,
					"peer.ip":     c.RealIP(),
				}),
			)
			defer end()

			c.SetRequest(req.WithContext(ctx))

			// Backwards-compat shim: callers that still do
			// c.Get("dry_logger").(drylog.Logger).Infof(...) keep working
			// for one release. Remove in the follow-up that drops the
			// pre-event-API access pattern.
			c.Set("dry_logger", drylog.With(map[string]any{"request_id": requestID}))

			err := next(c)

			resp := c.Response()
			drylog.SetAttrs(ctx, map[string]any{
				"http.status":    resp.Status,
				"http.bytes_out": resp.Size,
				"http.bytes_in":  req.ContentLength,
				"http.route":     c.Path(),
			})
			if err != nil {
				drylog.Err(ctx, err)
			}
			return err
		}
	}
}
