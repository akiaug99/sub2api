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
		Credentials: map[string]any{"access_token": "SENSITIVE_VALUE_FIXTURE", "plan_type": "k12"},
		Extra:       map[string]any{"proxy_url": "http://proxy.invalid", "codex_5h_window_minutes": 300},
		Status:      StatusActive, Schedulable: true,
		Groups: []*Group{{ID: 3, Name: "primary", Platform: "openai"}},
	}
	usage := &UsageInfo{SubscriptionTier: "plus", FiveHour: &UsageProgress{Utilization: 20, ResetsAt: &resetAt}}

	item := mapPoolMonitorAccount(account, usage, now)
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	text := string(raw)
	for _, forbidden := range []string{"private@example.invalid", "access_token", "SENSITIVE_VALUE_FIXTURE", "proxy_url", "proxy.invalid"} {
		require.NotContains(t, text, forbidden)
	}
	require.Equal(t, "p***@e***.invalid", item.MaskedLabel)
	require.Equal(t, "k12", item.Plan)
	require.Equal(t, []string{"primary"}, item.Groups)
	require.Len(t, item.QuotaWindows, 1)
}

func TestPoolMonitorAccountHidesUnconfirmedOpenAIWindows(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 100},
	}
	for _, tc := range []struct {
		name  string
		extra map[string]any
	}{
		{name: "missing window lengths"},
		{name: "zero and noncanonical window lengths", extra: map[string]any{
			"codex_5h_window_minutes": 0,
			"codex_7d_window_minutes": 43200,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := Account{
				ID: 10, Name: "free", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
				Credentials: map[string]any{"plan_type": "free"}, Extra: tc.extra,
				Status: StatusActive, Schedulable: true,
			}

			item := mapPoolMonitorAccount(account, usage, now)

			require.Empty(t, item.QuotaWindows)
			require.True(t, item.Limited, "hidden exhausted windows must still keep the account limited")
		})
	}
}

func TestPoolMonitorSnapshotUsesCredentialAndParentPlanTypes(t *testing.T) {
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	parentID := int64(1)
	accounts := []Account{
		{ID: parentID, Name: "pro", Platform: "openai", Type: "oauth", Credentials: map[string]any{"plan_type": "pro"}, Status: StatusActive, Schedulable: true},
		{ID: 2, Name: "shadow", Platform: "openai", Type: "oauth", ParentAccountID: &parentID, Status: StatusActive, Schedulable: true},
		{ID: 3, Name: "free", Platform: "openai", Type: "oauth", Credentials: map[string]any{"plan_type": "free"}, Status: StatusActive, Schedulable: true},
		{ID: 4, Name: "plus", Platform: "openai", Type: "oauth", Credentials: map[string]any{"plan_type": "plus"}, Status: StatusActive, Schedulable: true},
	}
	bridge := newPoolMonitorBridgeForTest(&config.Config{PoolMonitor: config.PoolMonitorConfig{Enabled: true}}, func() time.Time { return now },
		func(_ context.Context, _, pageSize int) ([]Account, int64, error) {
			require.Equal(t, 500, pageSize)
			return accounts, int64(len(accounts)), nil
		},
		func(_ context.Context, accountID int64, _ bool) (*UsageInfo, error) {
			if accountID == 2 {
				return &UsageInfo{SubscriptionTier: "stale-tier"}, nil
			}
			return &UsageInfo{}, nil
		})

	snapshot, err := bridge.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"pro", "pro", "free", "plus"}, []string{
		snapshot.Accounts[0].Plan,
		snapshot.Accounts[1].Plan,
		snapshot.Accounts[2].Plan,
		snapshot.Accounts[3].Plan,
	})
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
