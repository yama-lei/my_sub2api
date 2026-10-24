package service

import (
	"context"
	"errors"
)

func (s *AccountUsageService) SetQoderService(qoder *QoderService) {
	s.qoderService = qoder
}

// Both individual and batch usage queries use the same Credits cache and refresh policy.
func (s *AccountUsageService) getQoderUsage(ctx context.Context, account *Account, force bool) (*UsageInfo, error) {
	if s.qoderService == nil {
		return nil, errors.New("未配置 Qoder 用量服务")
	}
	row, err := s.qoderService.AccountCredits(ctx, account, force)
	if err != nil {
		return nil, err
	}
	info := &UsageInfo{Source: "active", QoderCredits: &row, Error: row.Error}
	if row.Credits != nil {
		info.UpdatedAt = &row.Credits.FetchedAt
		info.SubscriptionTier = row.Credits.Plan
	}
	return info, nil
}
