package middleware

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"
)

// RateLimiter returns a per-workspace (or per-IP) rate-limiting middleware.
// rps is the sustained request rate; burst allows short spikes above that.
func RateLimiter(rps float64, burst int) echo.MiddlewareFunc {
	store := middleware.NewRateLimiterMemoryStoreWithConfig(
		middleware.RateLimiterMemoryStoreConfig{
			Rate:  rate.Limit(rps),
			Burst: burst,
		},
	)

	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			if wsID := c.Request().Header.Get("X-Workspace-ID"); wsID != "" {
				return wsID, nil
			}
			return c.RealIP(), nil
		},
		DenyHandler: func(c echo.Context, id string, err error) error {
			return c.JSON(http.StatusTooManyRequests, map[string]any{
				"type":   "https://aflow.dev/errors/rate-limited",
				"title":  "Too Many Requests",
				"status": http.StatusTooManyRequests,
				"detail": fmt.Sprintf("Rate limit exceeded for %s. Slow down and retry.", id),
			})
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.JSON(http.StatusInternalServerError, map[string]any{
				"type":   "https://aflow.dev/errors/internal-error",
				"title":  "Internal Server Error",
				"status": http.StatusInternalServerError,
			})
		},
	})
}
