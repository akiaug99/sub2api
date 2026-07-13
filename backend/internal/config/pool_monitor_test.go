package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatePoolMonitorConfigRequiresSafeEndpointsAndSecret(t *testing.T) {
	valid := PoolMonitorConfig{
		Enabled: true, SharedSecret: strings.Repeat("x", 32),
		IntegratedExchangeURL: "https://sub2api.example.invalid/pool-admin/api/admin/v1/auth/exchange",
		StandaloneExchangeURL: "https://pool.example.invalid/api/admin/v1/auth/exchange",
	}
	require.NoError(t, validatePoolMonitorConfig(valid))

	invalid := valid
	invalid.SharedSecret = "short"
	require.ErrorContains(t, validatePoolMonitorConfig(invalid), "shared_secret")
	invalid = valid
	invalid.StandaloneExchangeURL = "http://pool.example.invalid/api/admin/v1/auth/exchange"
	require.ErrorContains(t, validatePoolMonitorConfig(invalid), "https")
}

func TestValidatePoolMonitorConfigAllowsLoopbackHTTPForLocalDevelopment(t *testing.T) {
	require.NoError(t, validatePoolMonitorConfig(PoolMonitorConfig{
		Enabled: true, SharedSecret: strings.Repeat("x", 32),
		IntegratedExchangeURL: "http://localhost:8095/pool-admin/api/admin/v1/auth/exchange",
		StandaloneExchangeURL: "http://127.0.0.1:8095/api/admin/v1/auth/exchange",
	}))
}
