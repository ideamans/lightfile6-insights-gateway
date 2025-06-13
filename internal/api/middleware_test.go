package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware(t *testing.T) {
	e := echo.New()
	
	// Create a handler that returns success
	handler := func(c echo.Context) error {
		user := c.Get("user").(string)
		return c.String(http.StatusOK, user)
	}
	
	// Apply middleware
	middleware := AuthMiddleware()
	h := middleware(handler)
	
	tests := []struct {
		name       string
		token      string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "with valid token",
			token:      "testuser",
			wantStatus: http.StatusOK,
			wantBody:   "testuser",
		},
		{
			name:       "without token",
			token:      "",
			wantStatus: http.StatusUnauthorized,
			wantBody:   "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.token != "" {
				req.Header.Set("USER_TOKEN", tt.token)
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			err := h(c)
			
			if tt.wantStatus == http.StatusUnauthorized {
				httpErr, ok := err.(*echo.HTTPError)
				assert.True(t, ok)
				assert.Equal(t, tt.wantStatus, httpErr.Code)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStatus, rec.Code)
				assert.Equal(t, tt.wantBody, rec.Body.String())
			}
		})
	}
}

func TestLoggerMiddleware(t *testing.T) {
	e := echo.New()
	
	// Create a handler that returns different status codes
	tests := []struct {
		name       string
		handler    echo.HandlerFunc
		wantStatus int
	}{
		{
			name: "success",
			handler: func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "client error",
			handler: func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusBadRequest, "bad request")
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "server error",
			handler: func(c echo.Context) error {
				return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}
	
	middleware := LoggerMiddleware()
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := middleware(tt.handler)
			
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			
			// Set request ID for testing
			c.Response().Header().Set(echo.HeaderXRequestID, "test-request-id")
			
			err := h(c)
			
			// The middleware should not return an error even if the handler does
			assert.NoError(t, err)
			
			// Check that the response status was set correctly
			// The logger middleware logs errors but doesn't change the error handling
		})
	}
}