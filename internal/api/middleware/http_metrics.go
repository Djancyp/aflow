package middleware

import (
	"strconv"
	"time"

	"github.com/djan/aflow/internal/observability/metrics"
	"github.com/labstack/echo/v4"
)

// HTTPMetrics records request count and latency for every route.
// Uses c.Path() (the route template) so high-cardinality IDs don't explode label sets.
func HTTPMetrics(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)
		dur := time.Since(start).Seconds()

		path := c.Path()
		if path == "" {
			path = "unknown"
		}
		status := strconv.Itoa(c.Response().Status)
		method := c.Request().Method

		metrics.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(dur)

		return err
	}
}
