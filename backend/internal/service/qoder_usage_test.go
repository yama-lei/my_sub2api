package service

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type qoderUsageRepoStub struct{ *creditsRepoStub }

func (r qoderUsageRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	account, err := r.GetByID(ctx, ids[0])
	return []*Account{account}, err
}

func TestQoderUsesExistingIndividualAndBatchUsageQueries(t *testing.T) {
	var calls atomic.Int32
	s, repo, account := newCreditsTestService(func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		return quotaReply(req, http.StatusOK), nil
	})
	usageService := &AccountUsageService{accountRepo: qoderUsageRepoStub{repo}}
	usageService.SetQoderService(s)
	usage, err := usageService.GetUsage(context.Background(), account.ID)
	require.NoError(t, err)
	require.NotNil(t, usage.QoderCredits)
	require.Equal(t, 48.5, usage.QoderCredits.Credits.Subscription.Remaining)
	require.Equal(t, "active", usage.Source)
	require.Equal(t, int32(2), calls.Load())
	rows, failures, err := usageService.GetUsageBatch(context.Background(), []int64{account.ID, account.ID}, false)
	require.NoError(t, err)
	require.Empty(t, failures)
	require.Len(t, rows, 1)
	require.Equal(t, "cached", rows[account.ID].QoderCredits.State)
	require.Equal(t, int32(2), calls.Load(), "batch requests must reuse the authorization-scoped cache")
}
