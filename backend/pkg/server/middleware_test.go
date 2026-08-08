package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestFrontendCacheControlMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{name: "spa route revalidates html", path: "/flows", expected: "no-cache"},
		{name: "root revalidates html", path: "/", expected: "no-cache"},
		{name: "hashed assets are immutable", path: "/assets/flows-a1b2c3.js", expected: "public, max-age=31536000, immutable"},
		{name: "api cache policy is unchanged", path: "/api/v1/health", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(frontendCacheControlMiddleware())
			router.GET("/*path", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			assert.Equal(t, tt.expected, response.Header().Get("Cache-Control"))
		})
	}
}
