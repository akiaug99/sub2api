package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func NewPoolMonitorHMACMiddleware(sharedSecret string, now func() time.Time) gin.HandlerFunc {
	requestKey := derivePoolMonitorRequestKey([]byte(sharedSecret))
	return func(c *gin.Context) {
		timestamp := c.GetHeader("X-Pool-Timestamp")
		signature := c.GetHeader("X-Pool-Signature")
		signedAt, err := time.Parse(time.RFC3339, timestamp)
		delta := now().UTC().Sub(signedAt)
		if err != nil || delta < -60*time.Second || delta > 60*time.Second || !verifyPoolMonitorRequest(requestKey, c.Request.Method, c.Request.URL.Path, timestamp, signature) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func derivePoolMonitorRequestKey(master []byte) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte("pool-monitor/v1/service-request"))
	return mac.Sum(nil)
}

func signPoolMonitorRequest(sharedSecret, method, requestPath, timestamp string) string {
	key := derivePoolMonitorRequestKey([]byte(sharedSecret))
	sum := sha256.Sum256(nil)
	canonical := strings.ToUpper(method) + "\n" + requestPath + "\n" + timestamp + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyPoolMonitorRequest(key []byte, method, requestPath, timestamp, signature string) bool {
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(nil)
	canonical := strings.ToUpper(method) + "\n" + requestPath + "\n" + timestamp + "\n" + hex.EncodeToString(sum[:])
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(canonical))
	return hmac.Equal(mac.Sum(nil), provided)
}
