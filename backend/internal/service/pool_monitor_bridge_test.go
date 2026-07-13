package service

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPoolMonitorSnapshotOmitsSensitiveAccountFields(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	resetAt := now.Add(5 * time.Hour)
	account := Account{
		ID: 9, Name: "private@example.invalid", Platform: "openai", Type: "oauth",
		Credentials: map[string]any{"access_token": "SENSITIVE_VALUE_FIXTURE"},
		Extra:       map[string]any{"proxy_url": "http://proxy.invalid"},
		Status:      StatusActive, Schedulable: true,
		Groups: []*Group{{ID: 3, Name: "primary", Platform: "openai"}},
	}
	usage := &UsageInfo{FiveHour: &UsageProgress{Utilization: 20, ResetsAt: &resetAt}}

	item := mapPoolMonitorAccount(account, usage, now)
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	text := string(raw)
	for _, forbidden := range []string{"private@example.invalid", "access_token", "SENSITIVE_VALUE_FIXTURE", "proxy_url", "proxy.invalid"} {
		require.NotContains(t, text, forbidden)
	}
	require.Equal(t, "p***@e***.invalid", item.MaskedLabel)
	require.Equal(t, []string{"primary"}, item.Groups)
	require.Len(t, item.QuotaWindows, 1)
}

func TestPoolMonitorBridgePaginatesAccountsAndUsesPassiveUsage(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	pages := 0
	forceValues := make([]bool, 0)
	var forceMu sync.Mutex
	bridge := newPoolMonitorBridgeForTest(&config.Config{PoolMonitor: config.PoolMonitorConfig{Enabled: true}}, func() time.Time { return now },
		func(_ context.Context, page, pageSize int) ([]Account, int64, error) {
			pages++
			require.Equal(t, 500, pageSize)
			if page == 1 {
				return []Account{{ID: 1, Name: "one", Platform: "openai", Status: StatusActive, Schedulable: true}}, 2, nil
			}
			return []Account{{ID: 2, Name: "two", Platform: "kiro", Status: StatusActive, Schedulable: true}}, 2, nil
		},
		func(_ context.Context, _ int64, force bool) (*UsageInfo, error) {
			forceMu.Lock()
			forceValues = append(forceValues, force)
			forceMu.Unlock()
			return &UsageInfo{}, nil
		})

	snapshot, err := bridge.Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot.Accounts, 2)
	require.Equal(t, 2, pages)
	require.Equal(t, []bool{false, false}, forceValues)
}

func TestPoolMonitorTicketContainsNoSharedSecret(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	secret := strings.Repeat("x", 32)
	bridge := newPoolMonitorBridgeForTest(&config.Config{PoolMonitor: config.PoolMonitorConfig{
		Enabled: true, SharedSecret: secret,
		IntegratedExchangeURL: "https://sub2api.example.invalid/pool-admin/api/admin/v1/auth/exchange",
		StandaloneExchangeURL: "https://pool.example.invalid/api/admin/v1/auth/exchange",
	}}, func() time.Time { return now }, nil, nil)

	result, err := bridge.CreateSSOTicket(7, "integrated")
	require.NoError(t, err)
	require.Equal(t, "https://sub2api.example.invalid/pool-admin/api/admin/v1/auth/exchange", result.TargetURL)
	require.NotEmpty(t, result.Ticket)
	require.NotContains(t, result.Ticket, secret)
}
