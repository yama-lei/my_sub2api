package service

import (
	"context"
	"sort"
	"time"
)

// Account states reported by the relay group status page.
const (
	RelayAccountAvailable     = "available"
	RelayAccountRateLimited   = "rate_limited"
	RelayAccountOverloaded    = "overloaded"
	RelayAccountCooling       = "cooling"
	RelayAccountQuotaExceeded = "quota_exceeded"
	RelayAccountExpired       = "expired"
	RelayAccountPaused        = "paused"
	RelayAccountError         = "error"
	RelayAccountDisabled      = "disabled"
)

// Group kinds reported by the relay group status page.
const (
	RelayGroupPrivate  = "private"
	RelayGroupPublic   = "public_relay"
	RelayGroupStandard = "standard"
)

// RelayAccountStatus describes one account in a group. Accounts owned by other
// users or by the platform are anonymous: only platform, type and state are set.
type RelayAccountStatus struct {
	Mine         bool       `json:"mine"`
	ID           int64      `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Platform     string     `json:"platform"`
	Type         string     `json:"type"`
	State        string     `json:"state"`
	Until        *time.Time `json:"until,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	Reason       string     `json:"reason,omitempty"`
}

type RelayGroupStatus struct {
	ID               int64                `json:"id"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	Platform         string               `json:"platform"`
	Kind             string               `json:"kind"`
	Status           string               `json:"status"`
	SubscriptionType string               `json:"subscription_type"`
	RateMultiplier   float64              `json:"rate_multiplier"`
	SupplierRate     float64              `json:"supplier_rate"`
	Total            int                  `json:"total"`
	Available        int                  `json:"available"`
	Mine             int                  `json:"mine"`
	Accounts         []RelayAccountStatus `json:"accounts"`
}

// GroupStatus lists every group the user can call (own private groups first)
// with the live state of each account bound to it.
func (s *RelayService) GroupStatus(ctx context.Context, userID int64) ([]RelayGroupStatus, error) {
	private, err := s.Groups(ctx, userID)
	if err != nil {
		return nil, err
	}
	available, err := s.keys.GetAvailableGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(private)+len(available))
	groups := make([]Group, 0, len(private)+len(available))
	for _, list := range [][]Group{private, available} {
		for _, g := range list {
			if !seen[g.ID] {
				seen[g.ID] = true
				groups = append(groups, g)
			}
		}
	}
	rank := map[string]int{RelayGroupPrivate: 0, RelayGroupPublic: 1, RelayGroupStandard: 2}
	out := make([]RelayGroupStatus, 0, len(groups))
	now := time.Now()
	for i := range groups {
		g := &groups[i]
		accounts, err := s.accounts.ListAllWithFilters(ctx, "", "", "", "", g.ID, "")
		if err != nil {
			return nil, err
		}
		out = append(out, buildRelayGroupStatus(g, accounts, userID, now))
	}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Kind] < rank[out[j].Kind] })
	return out, nil
}

// buildRelayGroupStatus summarizes one group. Only the caller's own accounts carry
// identifying details; error text from other suppliers may contain their account data.
func buildRelayGroupStatus(g *Group, accounts []Account, userID int64, now time.Time) RelayGroupStatus {
	status := RelayGroupStatus{ID: g.ID, Name: g.Name, Description: g.Description, Platform: g.Platform,
		Kind: relayGroupKind(g), Status: g.Status, SubscriptionType: g.SubscriptionType,
		RateMultiplier: g.RateMultiplier, SupplierRate: g.SupplierRate,
		Accounts: make([]RelayAccountStatus, 0, len(accounts))}
	for j := range accounts {
		a := &accounts[j]
		row := relayAccountStatus(a, now)
		if a.OwnerUserID != nil && *a.OwnerUserID == userID {
			row.Mine, row.ID, row.Name = true, a.ID, a.Name
			row.ErrorMessage, row.Reason = a.ErrorMessage, a.TempUnschedulableReason
			status.Mine++
		}
		if row.State == RelayAccountAvailable {
			status.Available++
		}
		status.Accounts = append(status.Accounts, row)
	}
	status.Total = len(status.Accounts)
	return status
}

func relayGroupKind(g *Group) string {
	switch {
	case g.IsPrivate():
		return RelayGroupPrivate
	case g.IsPublicRelay():
		return RelayGroupPublic
	default:
		return RelayGroupStandard
	}
}

// relayAccountStatus mirrors the checks in Account.IsSchedulable, reporting the
// first reason an account cannot currently receive requests.
func relayAccountStatus(a *Account, now time.Time) RelayAccountStatus {
	row := RelayAccountStatus{Platform: a.Platform, Type: a.Type}
	until := func(t *time.Time) bool { return t != nil && now.Before(*t) }
	switch {
	case a.Status == StatusError:
		row.State = RelayAccountError
	case !a.IsActive():
		row.State = RelayAccountDisabled
	case !a.Schedulable:
		row.State = RelayAccountPaused
	case a.AutoPauseOnExpired && a.ExpiresAt != nil && !now.Before(*a.ExpiresAt):
		row.State = RelayAccountExpired
	case until(a.OverloadUntil):
		row.State, row.Until = RelayAccountOverloaded, a.OverloadUntil
	case until(a.RateLimitResetAt):
		row.State, row.Until = RelayAccountRateLimited, a.RateLimitResetAt
	case until(a.TempUnschedulableUntil):
		row.State, row.Until = RelayAccountCooling, a.TempUnschedulableUntil
	case a.IsAPIKeyOrBedrock() && a.IsQuotaExceeded():
		row.State = RelayAccountQuotaExceeded
	default:
		row.State = RelayAccountAvailable
	}
	return row
}
