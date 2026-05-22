package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

type accountUsageCodexProbeRepo struct {
	stubOpenAIAccountRepo
	updateExtraCh chan map[string]any
	rateLimitCh   chan time.Time
	clearLimitCh  chan int64
}

func (r *accountUsageCodexProbeRepo) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	if r.updateExtraCh != nil {
		copied := make(map[string]any, len(updates))
		for k, v := range updates {
			copied[k] = v
		}
		r.updateExtraCh <- copied
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) SetRateLimited(_ context.Context, _ int64, resetAt time.Time) error {
	if r.rateLimitCh != nil {
		r.rateLimitCh <- resetAt
	}
	return nil
}

func (r *accountUsageCodexProbeRepo) ClearRateLimit(_ context.Context, id int64) error {
	if r.clearLimitCh != nil {
		r.clearLimitCh <- id
	}
	return nil
}

type accountUsageProbeUpstream struct {
	response *http.Response
	err      error
	requests []*http.Request
}

func (u *accountUsageProbeUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return u.do(req)
}

func (u *accountUsageProbeUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.do(req)
}

func (u *accountUsageProbeUpstream) do(req *http.Request) (*http.Response, error) {
	u.requests = append(u.requests, req)
	if u.err != nil {
		return nil, u.err
	}
	if u.response == nil {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	return u.response, nil
}

func newAccountUsageProbeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestShouldRefreshOpenAICodexSnapshot(t *testing.T) {
	t.Parallel()

	rateLimitedUntil := time.Now().Add(5 * time.Minute)
	now := time.Now()
	usage := &UsageInfo{
		FiveHour: &UsageProgress{Utilization: 0},
		SevenDay: &UsageProgress{Utilization: 0},
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{RateLimitResetAt: &rateLimitedUntil}, usage, now) {
		t.Fatal("expected rate-limited account to force codex snapshot refresh")
	}

	if shouldRefreshOpenAICodexSnapshot(&Account{}, usage, now) {
		t.Fatal("expected complete non-rate-limited usage to skip codex snapshot refresh")
	}

	if !shouldRefreshOpenAICodexSnapshot(&Account{}, &UsageInfo{FiveHour: nil, SevenDay: &UsageProgress{}}, now) {
		t.Fatal("expected missing 5h snapshot to require refresh")
	}

	staleAt := now.Add(-(openAIProbeCacheTTL + time.Minute)).Format(time.RFC3339)
	if !shouldRefreshOpenAICodexSnapshot(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"openai_oauth_responses_websockets_v2_enabled": true,
			"codex_usage_updated_at":                       staleAt,
		},
	}, usage, now) {
		t.Fatal("expected stale ws snapshot to trigger refresh")
	}
}

func TestShouldVerifyOpenAICodexWeeklyLimitWithProbe(t *testing.T) {
	t.Parallel()

	now := time.Now()
	activeReset := now.Add(time.Hour)
	expiredReset := now.Add(-time.Hour)

	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	if !shouldVerifyOpenAICodexWeeklyLimitWithProbe(account, &UsageInfo{SevenDay: &UsageProgress{Utilization: 100, ResetsAt: &activeReset}}, now) {
		t.Fatal("expected active exhausted 7d snapshot to require verification probe")
	}
	if shouldVerifyOpenAICodexWeeklyLimitWithProbe(account, &UsageInfo{SevenDay: &UsageProgress{Utilization: 99.9, ResetsAt: &activeReset}}, now) {
		t.Fatal("did not expect non-exhausted 7d snapshot to require verification probe")
	}
	if shouldVerifyOpenAICodexWeeklyLimitWithProbe(account, &UsageInfo{SevenDay: &UsageProgress{Utilization: 100, ResetsAt: &expiredReset}}, now) {
		t.Fatal("did not expect expired 7d snapshot to require verification probe")
	}
	if shouldVerifyOpenAICodexWeeklyLimitWithProbe(&Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, &UsageInfo{SevenDay: &UsageProgress{Utilization: 100, ResetsAt: &activeReset}}, now) {
		t.Fatal("did not expect API key account to require codex verification probe")
	}
}

func TestExtractOpenAICodexProbeUpdatesAccepts429WithCodexHeaders(t *testing.T) {
	t.Parallel()

	headers := make(http.Header)
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")
	headers.Set("x-codex-secondary-used-percent", "100")
	headers.Set("x-codex-secondary-reset-after-seconds", "18000")
	headers.Set("x-codex-secondary-window-minutes", "300")

	updates, err := extractOpenAICodexProbeUpdates(&http.Response{StatusCode: http.StatusTooManyRequests, Header: headers})
	if err != nil {
		t.Fatalf("extractOpenAICodexProbeUpdates() error = %v", err)
	}
	if len(updates) == 0 {
		t.Fatal("expected codex probe updates from 429 headers")
	}
	if got := updates["codex_5h_used_percent"]; got != 100.0 {
		t.Fatalf("codex_5h_used_percent = %v, want 100", got)
	}
	if got := updates["codex_7d_used_percent"]; got != 100.0 {
		t.Fatalf("codex_7d_used_percent = %v, want 100", got)
	}
}

func TestAccountUsageService_GetOpenAIUsage_VerifiesExhaustedCodexSnapshotAndRateLimitsOn429(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(90 * time.Minute).Unix()
	resp := newAccountUsageProbeResponse(http.StatusTooManyRequests, `{"error":{"type":"usage_limit_reached","resets_at":`+fmt.Sprint(resetAt)+`}}`)

	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	upstream := &accountUsageProbeUpstream{response: resp}
	svc := &AccountUsageService{
		accountRepo:         repo,
		cache:               NewUsageCache(),
		openAIProbeUpstream: upstream,
	}
	account := &Account{
		ID:          701,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{"access_token": "test-token"},
		Extra: map[string]any{
			"codex_5h_used_percent":  1.0,
			"codex_5h_reset_at":      time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"codex_7d_used_percent":  100.0,
			"codex_7d_reset_at":      time.Now().Add(6 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"codex_usage_updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100 {
		t.Fatalf("expected exhausted 7d usage to remain visible, got %#v", usage.SevenDay)
	}
	if len(upstream.requests) != 1 {
		t.Fatalf("expected one codex verification probe, got %d", len(upstream.requests))
	}
	select {
	case got := <-repo.rateLimitCh:
		if got.Unix() != resetAt {
			t.Fatalf("rate limit reset = %v, want unix %d", got, resetAt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting for rate limit write timed out")
	}
}

func TestAccountUsageService_GetOpenAIUsage_VerifiesExhaustedCodexSnapshotAndClearsRateLimitOnSuccess(t *testing.T) {
	t.Parallel()

	resp := newAccountUsageProbeResponse(http.StatusOK, "")
	resp.Header.Set("x-codex-primary-used-percent", "88")
	resp.Header.Set("x-codex-primary-reset-after-seconds", "604800")
	resp.Header.Set("x-codex-primary-window-minutes", "10080")
	resp.Header.Set("x-codex-secondary-used-percent", "12")
	resp.Header.Set("x-codex-secondary-reset-after-seconds", "18000")
	resp.Header.Set("x-codex-secondary-window-minutes", "300")

	repo := &accountUsageCodexProbeRepo{
		clearLimitCh: make(chan int64, 1),
	}
	upstream := &accountUsageProbeUpstream{response: resp}
	svc := &AccountUsageService{
		accountRepo:         repo,
		cache:               NewUsageCache(),
		openAIProbeUpstream: upstream,
	}
	rateLimitedUntil := time.Now().Add(2 * time.Hour)
	account := &Account{
		ID:               702,
		Platform:         PlatformOpenAI,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		Schedulable:      true,
		Concurrency:      1,
		RateLimitResetAt: &rateLimitedUntil,
		Credentials:      map[string]any{"access_token": "test-token"},
		Extra: map[string]any{
			"codex_5h_used_percent":  1.0,
			"codex_5h_reset_at":      time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"codex_7d_used_percent":  100.0,
			"codex_7d_reset_at":      time.Now().Add(6 * 24 * time.Hour).UTC().Format(time.RFC3339),
			"codex_usage_updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 88 {
		t.Fatalf("expected refreshed 7d usage from successful probe, got %#v", usage.SevenDay)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("expected successful probe to clear runtime rate limit, got %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.clearLimitCh:
		if got != account.ID {
			t.Fatalf("cleared account id = %d, want %d", got, account.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("waiting for clear rate limit timed out")
	}
}

func TestAccountUsageService_PersistOpenAICodexProbeSnapshotOnlyUpdatesExtra(t *testing.T) {
	t.Parallel()

	repo := &accountUsageCodexProbeRepo{
		updateExtraCh: make(chan map[string]any, 1),
		rateLimitCh:   make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	svc.persistOpenAICodexProbeSnapshot(321, map[string]any{
		"codex_7d_used_percent": 100.0,
		"codex_7d_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
	})

	select {
	case updates := <-repo.updateExtraCh:
		if got := updates["codex_7d_used_percent"]; got != 100.0 {
			t.Fatalf("codex_7d_used_percent = %v, want 100", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("等待 codex 探测快照写入 extra 超时")
	}

	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将探测快照写入运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestAccountUsageService_GetOpenAIUsage_DoesNotPromoteCodexExtraToRateLimit(t *testing.T) {
	t.Parallel()

	resetAt := time.Now().Add(6 * 24 * time.Hour).UTC().Truncate(time.Second)
	repo := &accountUsageCodexProbeRepo{
		rateLimitCh: make(chan time.Time, 1),
	}
	svc := &AccountUsageService{accountRepo: repo}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Extra: map[string]any{
			"codex_5h_used_percent": 1.0,
			"codex_5h_reset_at":     time.Now().Add(2 * time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
			"codex_7d_used_percent": 100.0,
			"codex_7d_reset_at":     resetAt.Format(time.RFC3339),
		},
	}

	usage, err := svc.getOpenAIUsage(context.Background(), account)
	if err != nil {
		t.Fatalf("getOpenAIUsage() error = %v", err)
	}
	if usage.SevenDay == nil || usage.SevenDay.Utilization != 100.0 {
		t.Fatalf("预期 7 天用量仍然可见，实际为 %#v", usage.SevenDay)
	}
	if account.RateLimitResetAt != nil {
		t.Fatalf("不应让已耗尽的 codex extra 改写运行时限流状态: %v", account.RateLimitResetAt)
	}
	select {
	case got := <-repo.rateLimitCh:
		t.Fatalf("不应将已耗尽的 codex extra 持久化为运行时限流状态: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBuildCodexUsageProgressFromExtra_ZerosExpiredWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC)

	t.Run("expired 5h window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     "2026-03-16T10:00:00Z", // 2h ago
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired window, got %v", progress.Utilization)
		}
		if progress.RemainingSeconds != 0 {
			t.Fatalf("expected RemainingSeconds=0, got %v", progress.RemainingSeconds)
		}
	})

	t.Run("active 5h window keeps utilization", func(t *testing.T) {
		resetAt := now.Add(2 * time.Hour).Format(time.RFC3339)
		extra := map[string]any{
			"codex_5h_used_percent": 42.0,
			"codex_5h_reset_at":     resetAt,
		}
		progress := buildCodexUsageProgressFromExtra(extra, "5h", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 42.0 {
			t.Fatalf("expected Utilization=42, got %v", progress.Utilization)
		}
	})

	t.Run("expired 7d window zeroes utilization", func(t *testing.T) {
		extra := map[string]any{
			"codex_7d_used_percent": 88.0,
			"codex_7d_reset_at":     "2026-03-15T00:00:00Z", // yesterday
		}
		progress := buildCodexUsageProgressFromExtra(extra, "7d", now)
		if progress == nil {
			t.Fatal("expected non-nil progress")
		}
		if progress.Utilization != 0 {
			t.Fatalf("expected Utilization=0 for expired 7d window, got %v", progress.Utilization)
		}
	})
}
