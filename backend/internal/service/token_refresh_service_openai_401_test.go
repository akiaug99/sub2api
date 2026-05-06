//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestTokenRefreshService_RefreshWithRetry_OpenAI401TempUnschedulableOneDay(t *testing.T) {
	repo := &tokenRefreshAccountRepo{}
	invalidator := &tokenCacheInvalidatorStub{}
	cfg := &config.Config{
		TokenRefresh: config.TokenRefreshConfig{
			MaxRetries:          3,
			RetryBackoffSeconds: 0,
		},
	}
	service := NewTokenRefreshService(repo, nil, nil, nil, nil, invalidator, nil, cfg, nil)
	account := &Account{
		ID:       120,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
	refresher := &tokenRefresherStub{
		err: infraerrors.Newf(http.StatusUnauthorized, "OPENAI_OAUTH_TOKEN_REFRESH_FAILED", "token refresh failed: status 401, body: {\"error\":{\"code\":\"refresh_token_reused\"}}"),
	}

	before := time.Now()
	err := service.refreshWithRetry(context.Background(), account, refresher, refresher, time.Hour)
	after := time.Now()

	require.Error(t, err)
	require.Equal(t, 1, refresher.refreshCalls, "401 refresh failure should not retry")
	require.Equal(t, 1, repo.setTempUnschedCalls, "401 refresh failure should set temp unschedulable")
	require.Equal(t, 0, repo.setErrorCalls, "401 refresh failure should not permanently mark error on first hit")
	require.Equal(t, 0, repo.updateCalls)
	require.Contains(t, repo.lastTempUnschedReason, "token refresh unauthorized cooldown")
	require.Contains(t, repo.lastTempUnschedReason, "refresh_token_reused")
	require.False(t, repo.lastTempUnschedUntil.Before(before.Add(24*time.Hour-2*time.Second)))
	require.False(t, repo.lastTempUnschedUntil.After(after.Add(24*time.Hour+2*time.Second)))
}

func TestTokenRefreshService_ProcessRefresh_SkipsUnauthorizedCooldownAccounts(t *testing.T) {
	until := time.Now().Add(24 * time.Hour)
	account := Account{
		ID:                     121,
		Name:                   "cooldown-account",
		Platform:               PlatformOpenAI,
		Type:                   AccountTypeOAuth,
		Status:                 StatusActive,
		TempUnschedulableUntil: &until,
		TempUnschedulableReason: tokenRefreshUnauthorizedCooldownReasonPrefix +
			"error: code=401 reason=\"OPENAI_OAUTH_TOKEN_REFRESH_FAILED\" message=\"refresh_token_reused\"",
	}
	repo := &tokenRefreshAccountRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accounts: []Account{account},
		},
	}
	refresher := &tokenRefresherStub{
		err: errors.New("should not be called"),
	}
	service := &TokenRefreshService{
		accountRepo:   repo,
		refreshers:    []TokenRefresher{refresher},
		executors:     []OAuthRefreshExecutor{refresher},
		refreshPolicy: DefaultBackgroundRefreshPolicy(),
		cfg: &config.TokenRefreshConfig{
			MaxRetries:          1,
			RetryBackoffSeconds: 0,
		},
	}

	service.processRefresh()
	require.Equal(t, 0, refresher.refreshCalls, "accounts in active unauthorized cooldown should be skipped by background refresh")
	require.Equal(t, 0, repo.setTempUnschedCalls)
	require.Equal(t, 0, repo.setErrorCalls)
}
