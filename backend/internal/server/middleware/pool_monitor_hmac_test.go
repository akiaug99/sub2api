package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPoolMonitorHMACRejectsOldTimestamp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	secret := strings.Repeat("x", 32)
	called := false
	router := gin.New()
	router.Use(NewPoolMonitorHMACMiddleware(secret, func() time.Time { return now }))
	router.GET("/api/v1/internal/pool-monitor/accounts", func(c *gin.Context) { called = true; c.Status(http.StatusOK) })
	timestamp := now.Add(-61 * time.Second).UTC().Format(time.RFC3339)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/internal/pool-monitor/accounts", nil)
	request.Header.Set("X-Pool-Timestamp", timestamp)
	request.Header.Set("X-Pool-Signature", signPoolMonitorRequest(secret, request.Method, request.URL.Path, timestamp))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.False(t, called)
}

func TestPoolMonitorHMACAcceptsMatchingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	secret := strings.Repeat("x", 32)
	router := gin.New()
	router.Use(NewPoolMonitorHMACMiddleware(secret, func() time.Time { return now }))
	router.GET("/safe", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	timestamp := now.UTC().Format(time.RFC3339)
	request := httptest.NewRequest(http.MethodGet, "/safe", nil)
	request.Header.Set("X-Pool-Timestamp", timestamp)
	request.Header.Set("X-Pool-Signature", signPoolMonitorRequest(secret, request.Method, request.URL.Path, timestamp))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNoContent, recorder.Code)
}
