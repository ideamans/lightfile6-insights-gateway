package api

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// AuthMiddleware validates the USER_TOKEN header
func AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := c.Request().Header.Get("USER_TOKEN")
			if token == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "USER_TOKEN header is required")
			}

			// For now, USER_TOKEN is the username
			c.Set("user", token)
			return next(c)
		}
	}
}

// LoggerMiddleware logs HTTP requests
func LoggerMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Process request
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			// Log request
			req := c.Request()
			res := c.Response()
			
			logger := log.Info()
			if res.Status >= 400 {
				logger = log.Error()
			}

			logger.
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Int("status", res.Status).
				Dur("latency", time.Since(start)).
				Str("remote_ip", c.RealIP()).
				Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
				Msg("HTTP request processed")

			return nil
		}
	}
}