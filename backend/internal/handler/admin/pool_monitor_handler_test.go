package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPoolMonitorSSOTicketRequiresAdminContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{PoolMonitor: config.PoolMonitorConfig{
		Enabled: true, SharedSecret: strings.Repeat("x", 32),
		IntegratedExchangeURL: "https://sub2api.example.invalid/pool-admin/api/admin/v1/auth/exchange",
		StandaloneExchangeURL: "https://pool.example.invalid/api/admin/v1/auth/exchange",
	}}
	handler := NewPoolMonitorHandler(service.NewPoolMonitorBridge(nil, nil, cfg), cfg)
	router := gin.New()
	router.POST("/ticket", handler.CreateSSOTicket)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/ticket", strings.NewReader(`{"target":"standalone"}`)))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestPoolMonitorSSOTicketAcceptsAdminContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{PoolMonitor: config.PoolMonitorConfig{
		Enabled: true, SharedSecret: strings.Repeat("x", 32),
		IntegratedExchangeURL: "https://sub2api.example.invalid/pool-admin/api/admin/v1/auth/exchange",
		StandaloneExchangeURL: "https://pool.example.invalid/api/admin/v1/auth/exchange",
	}}
	handler := NewPoolMonitorHandler(service.NewPoolMonitorBridge(nil, nil, cfg), cfg)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 7})
		c.Set(string(servermiddleware.ContextKeyUserRole), "admin")
		c.Next()
	})
	router.POST("/ticket", handler.CreateSSOTicket)
	request := httptest.NewRequest(http.MethodPost, "/ticket", strings.NewReader(`{"target":"standalone"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), strings.Repeat("x", 32))
}
