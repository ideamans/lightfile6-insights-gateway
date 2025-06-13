package api

import (
	"io"

	"github.com/labstack/echo/v4"
)

// readRequestBody reads and returns the request body
func readRequestBody(c echo.Context) ([]byte, error) {
	return io.ReadAll(c.Request().Body)
}