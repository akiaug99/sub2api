package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

var ErrPoolMonitorDisabled = errors.New("pool monitor is disabled")

type PoolMonitorQuotaWindow struct {
	Name      string     `json:"name"`
	Used      float64    `json:"used"`
	Limit     float64    `json:"limit"`
	Remaining float64    `json:"remaining"`
	ResetAt   *time.Time `json:"resetAt,omitempty"`
}

type PoolMonitorAccount struct {
	ID             string                   `json:"id"`
	MaskedLabel    string                   `json:"maskedLabel"`
	Platform       string                   `json:"platform"`
	Type           string                   `json:"type"`
	Plan           string                   `json:"plan,omitempty"`
	Status         string                   `json:"status"`
	Schedulable    bool                     `json:"schedulable"`
	Groups         []string                 `json:"groups"`
	TokenExpiresAt *time.Time               `json:"tokenExpiresAt,omitempty"`
	Limited        bool                     `json:"limited"`
	NoAccess       bool                     `json:"noAccess"`
	ErrorCategory  string                   `json:"errorCategory,omitempty"`
	QuotaWindows   []PoolMonitorQuotaWindow `json:"quotaWindows"`
}

type PoolMonitorSnapshot struct {
	GeneratedAt time.Time            `json:"generatedAt"`
	Accounts    []PoolMonitorAccount `json:"accounts"`
}

type PoolMonitorSSOTicketResult struct {
	TargetURL string `json:"target_url"`
	Ticket    string `json:"ticket"`
}

type poolMonitorListAccounts func(context.Context, int, int) ([]Account, int64, error)
type poolMonitorGetUsage func(context.Context, int64, bool) (*UsageInfo, error)

type PoolMonitorBridge struct {
	cfg          *config.Config
	now          func() time.Time
	listAccounts poolMonitorListAccounts
	getUsage     poolMonitorGetUsage
}

func NewPoolMonitorBridge(adminService AdminService, accountUsageService *AccountUsageService, cfg *config.Config) *PoolMonitorBridge {
	var list poolMonitorListAccounts
	if adminService != nil {
		list = func(ctx context.Context, page, pageSize int) ([]Account, int64, error) {
			return adminService.ListAccounts(ctx, page, pageSize, "", "", "", "", 0, "", "id", "asc")
		}
	}
	var usage poolMonitorGetUsage
	if accountUsageService != nil {
		usage = func(ctx context.Context, accountID int64, force bool) (*UsageInfo, error) {
			return accountUsageService.GetUsage(ctx, accountID, force)
		}
	}
	return newPoolMonitorBridgeForTest(cfg, time.Now, list, usage)
}

func newPoolMonitorBridgeForTest(cfg *config.Config, now func() time.Time, list poolMonitorListAccounts, usage poolMonitorGetUsage) *PoolMonitorBridge {
	return &PoolMonitorBridge{cfg: cfg, now: now, listAccounts: list, getUsage: usage}
}

func (b *PoolMonitorBridge) Snapshot(ctx context.Context) (PoolMonitorSnapshot, error) {
	if b.cfg == nil || !b.cfg.PoolMonitor.Enabled {
		return PoolMonitorSnapshot{}, ErrPoolMonitorDisabled
	}
	if b.listAccounts == nil {
		return PoolMonitorSnapshot{}, errors.New("pool monitor account reader unavailable")
	}
	accounts := make([]Account, 0)
	for page := 1; ; page++ {
		items, total, err := b.listAccounts(ctx, page, 500)
		if err != nil {
			return PoolMonitorSnapshot{}, fmt.Errorf("list monitored accounts: %w", err)
		}
		accounts = append(accounts, items...)
		if len(accounts) >= int(total) || len(items) == 0 {
			break
		}
	}

	usages := make([]*UsageInfo, len(accounts))
	usageErrors := make([]bool, len(accounts))
	if b.getUsage != nil {
		jobs := make(chan int)
		var wg sync.WaitGroup
		workers := 8
		if len(accounts) < workers {
			workers = len(accounts)
		}
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for index := range jobs {
					usage, err := b.getUsage(ctx, accounts[index].ID, false)
					if err != nil {
						usageErrors[index] = true
						continue
					}
					usages[index] = usage
				}
			}()
		}
		for index := range accounts {
			select {
			case jobs <- index:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return PoolMonitorSnapshot{}, ctx.Err()
			}
		}
		close(jobs)
		wg.Wait()
	}

	now := b.now().UTC()
	credentialPlans := make(map[int64]string, len(accounts))
	for _, account := range accounts {
		if plan := strings.TrimSpace(account.GetCredential("plan_type")); plan != "" {
			credentialPlans[account.ID] = plan
		}
	}
	result := PoolMonitorSnapshot{GeneratedAt: now, Accounts: make([]PoolMonitorAccount, len(accounts))}
	for index, account := range accounts {
		result.Accounts[index] = mapPoolMonitorAccount(account, usages[index], now)
		if strings.TrimSpace(account.GetCredential("plan_type")) == "" && account.ParentAccountID != nil {
			if parentPlan := credentialPlans[*account.ParentAccountID]; parentPlan != "" {
				result.Accounts[index].Plan = parentPlan
			}
		}
		if usageErrors[index] {
			result.Accounts[index].ErrorCategory = "usage_failed"
		}
	}
	return result, nil
}

func (b *PoolMonitorBridge) CreateSSOTicket(adminID int64, target string) (PoolMonitorSSOTicketResult, error) {
	if b.cfg == nil || !b.cfg.PoolMonitor.Enabled {
		return PoolMonitorSSOTicketResult{}, ErrPoolMonitorDisabled
	}
	var targetURL string
	switch target {
	case "integrated":
		targetURL = b.cfg.PoolMonitor.IntegratedExchangeURL
	case "standalone":
		targetURL = b.cfg.PoolMonitor.StandaloneExchangeURL
	default:
		return PoolMonitorSSOTicketResult{}, errors.New("invalid pool monitor target")
	}
	if adminID <= 0 || targetURL == "" {
		return PoolMonitorSSOTicketResult{}, errors.New("invalid pool monitor ticket request")
	}
	jtiBytes := make([]byte, 18)
	if _, err := rand.Read(jtiBytes); err != nil {
		return PoolMonitorSSOTicketResult{}, errors.New("create pool monitor ticket id")
	}
	now := b.now().UTC()
	payload := poolMonitorTicket{
		Version: 1, JTI: base64.RawURLEncoding.EncodeToString(jtiBytes), AdminID: adminID,
		Role: "admin", Target: target, Audience: "pool-monitor", IssuedAt: now.Unix(), ExpiresAt: now.Add(time.Minute).Unix(),
	}
	ticket, err := signPoolMonitorTicket(derivePoolMonitorKey([]byte(b.cfg.PoolMonitor.SharedSecret), "sso-ticket"), payload)
	if err != nil {
		return PoolMonitorSSOTicketResult{}, err
	}
	return PoolMonitorSSOTicketResult{TargetURL: targetURL, Ticket: ticket}, nil
}

func mapPoolMonitorAccount(account Account, usage *UsageInfo, now time.Time) PoolMonitorAccount {
	groups := make([]string, 0, len(account.Groups)+len(account.AccountGroups))
	seenGroups := make(map[string]struct{})
	for _, group := range account.Groups {
		if group != nil {
			appendPoolMonitorGroup(&groups, seenGroups, group.Name)
		}
	}
	for _, link := range account.AccountGroups {
		if link.Group != nil {
			appendPoolMonitorGroup(&groups, seenGroups, link.Group.Name)
		}
	}
	sort.Strings(groups)
	item := PoolMonitorAccount{
		ID: fmt.Sprintf("%d", account.ID), MaskedLabel: maskPoolMonitorLabel(account.Name),
		Platform: account.Platform, Type: account.Type, Status: account.Status, Schedulable: account.Schedulable,
		Plan: strings.TrimSpace(account.GetCredential("plan_type")), Groups: groups,
		TokenExpiresAt: account.ExpiresAt, QuotaWindows: make([]PoolMonitorQuotaWindow, 0),
	}
	if account.ExpiresAt != nil && !account.ExpiresAt.After(now) {
		item.Status = "expired"
	}
	item.Limited = activeAt(account.RateLimitResetAt, now) || activeAt(account.OverloadUntil, now) || activeAt(account.TempUnschedulableUntil, now)
	if usage != nil {
		if item.Plan == "" {
			item.Plan = strings.TrimSpace(usage.SubscriptionTier)
		}
		item.NoAccess = usage.NeedsReauth || usage.IsForbidden || usage.IsBanned
		item.ErrorCategory = allowPoolMonitorErrorCategory(usage.ErrorCode)
		appendUsageWindow := func(name string, progress *UsageProgress) {
			if progress == nil {
				return
			}
			window := PoolMonitorQuotaWindow{Name: name, ResetAt: progress.ResetsAt}
			if progress.LimitRequests > 0 {
				window.Used = float64(progress.UsedRequests)
				window.Limit = float64(progress.LimitRequests)
				window.Remaining = math.Max(0, window.Limit-window.Used)
			} else {
				window.Used = progress.Utilization
				window.Limit = 100
				window.Remaining = math.Max(0, 100-progress.Utilization)
			}
			if window.Remaining <= 0 {
				item.Limited = true
			}
			item.QuotaWindows = append(item.QuotaWindows, window)
		}
		appendUsageWindow("5h", usage.FiveHour)
		appendUsageWindow("7d", usage.SevenDay)
		appendUsageWindow("7d-sonnet", usage.SevenDaySonnet)
		appendUsageWindow("gemini-shared-daily", usage.GeminiSharedDaily)
		appendUsageWindow("gemini-pro-daily", usage.GeminiProDaily)
		appendUsageWindow("gemini-flash-daily", usage.GeminiFlashDaily)
		keys := make([]string, 0, len(usage.AntigravityQuota))
		for name := range usage.AntigravityQuota {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			quota := usage.AntigravityQuota[name]
			if quota == nil {
				continue
			}
			window := PoolMonitorQuotaWindow{Name: name, Used: float64(quota.Utilization), Limit: 100, Remaining: math.Max(0, 100-float64(quota.Utilization))}
			if resetAt, err := time.Parse(time.RFC3339, quota.ResetTime); err == nil {
				window.ResetAt = &resetAt
			}
			item.QuotaWindows = append(item.QuotaWindows, window)
		}
	}
	return item
}

func maskPoolMonitorLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未命名账号"
	}
	local, domain, found := strings.Cut(value, "@")
	if !found {
		return firstRune(value) + "***"
	}
	domainName, suffix, hasSuffix := strings.Cut(domain, ".")
	masked := firstRune(local) + "***@" + firstRune(domainName) + "***"
	if hasSuffix {
		masked += "." + suffix
	}
	return masked
}

func firstRune(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(value)
	return string(r)
}

func appendPoolMonitorGroup(groups *[]string, seen map[string]struct{}, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if _, ok := seen[value]; ok {
		return
	}
	seen[value] = struct{}{}
	*groups = append(*groups, value)
}

func activeAt(until *time.Time, now time.Time) bool {
	return until != nil && now.Before(*until)
}

func allowPoolMonitorErrorCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "forbidden", "unauthenticated", "rate_limited", "network_error":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		if strings.TrimSpace(value) != "" {
			return "usage_failed"
		}
		return ""
	}
}

type poolMonitorTicket struct {
	Version   int    `json:"version"`
	JTI       string `json:"jti"`
	AdminID   int64  `json:"adminId"`
	Role      string `json:"role"`
	Target    string `json:"target"`
	Audience  string `json:"audience"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

func derivePoolMonitorKey(master []byte, purpose string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte("pool-monitor/v1/" + purpose))
	return mac.Sum(nil)
}

func signPoolMonitorTicket(key []byte, ticket poolMonitorTicket) (string, error) {
	payload, err := json.Marshal(ticket)
	if err != nil {
		return "", errors.New("encode pool monitor ticket")
	}
	payloadPart := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payloadPart))
	return payloadPart + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
