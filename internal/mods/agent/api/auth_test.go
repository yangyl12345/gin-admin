package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LyricTian/gin-admin/v10/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAgentBearerAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("AGENT_API_KEY", "unit-test-agent-key")
	original := config.C.Agent.Enable
	config.C.Agent.Enable = true
	defer func() { config.C.Agent.Enable = original }()

	a := new(API)
	router := gin.New()
	router.GET("/protected", a.RequireEnabled(), a.Auth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, header := range []string{"", "Bearer wrong", "unit-test-agent-key", "Basic unit-test-agent-key"} {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		assert.Equal(t, http.StatusUnauthorized, response.Code, header)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer unit-test-agent-key")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	assert.Equal(t, http.StatusNoContent, response.Code)
}

func TestAgentDisabledBeforeAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	original := config.C.Agent.Enable
	config.C.Agent.Enable = false
	defer func() { config.C.Agent.Enable = original }()

	a := new(API)
	router := gin.New()
	router.GET("/protected", a.RequireEnabled(), a.Auth(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
}
