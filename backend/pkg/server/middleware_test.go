package router

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestResponseCompressionMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(responseCompressionMiddleware())
	router.GET("/history", func(c *gin.Context) {
		c.String(http.StatusOK, strings.Repeat("historical terminal output\n", 200))
	})

	request := httptest.NewRequest(http.MethodGet, "/history", nil)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, "gzip", response.Header().Get("Content-Encoding"))
	reader, err := gzip.NewReader(response.Body)
	assert.NoError(t, err)
	decompressed, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Contains(t, string(decompressed), "historical terminal output")
}

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
