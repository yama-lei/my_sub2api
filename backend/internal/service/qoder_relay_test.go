package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQoderRelayAuthorizationSessionsAreIsolatedByUserAndScope(t *testing.T) {
	s := qoderTestService()
	t.Cleanup(s.Stop)
	adminCtx := context.Background()
	relayCtx := WithRelayOwner(adminCtx, 1)
	session, err := s.Begin(relayCtx, 1, CreateAccountInput{Name: "private Qoder"}, 0)
	require.NoError(t, err)
	for _, ctx := range []context.Context{adminCtx, WithRelayOwner(adminCtx, 2)} {
		_, err = s.Poll(ctx, 1, session.SessionID)
		require.Error(t, err)
		_, err = s.Cancel(ctx, 1, session.SessionID)
		require.Error(t, err)
		_, err = s.Commit(ctx, 1, session.SessionID, func(context.Context, *CreateAccountInput) (*Account, error) {
			t.Fatal("a session from another scope was consumed")
			return nil, nil
		})
		require.Error(t, err)
	}
	_, err = s.Begin(relayCtx, 2, CreateAccountInput{Name: "wrong owner"}, 0)
	require.Error(t, err)
	_, err = s.Poll(relayCtx, 2, session.SessionID)
	require.Error(t, err)
	ready, err := s.Poll(relayCtx, 1, session.SessionID)
	require.NoError(t, err)
	require.Equal(t, "ready", ready.Status)
	result, err := s.Commit(relayCtx, 1, session.SessionID, func(ctx context.Context, input *CreateAccountInput) (*Account, error) {
		require.Equal(t, int64(1), RelayOwnerFromContext(ctx))
		require.Equal(t, PlatformQoder, input.Platform)
		require.Equal(t, AccountTypeOAuth, input.Type)
		require.NoError(t, ValidateRelayCredentials(input.Type, input.Credentials))
		return &Account{ID: 17}, nil
	})
	require.NoError(t, err)
	require.Equal(t, int64(17), result.AccountID)
}

func TestQoderRelayCannotReauthorizeOrReadAnotherOwnersCredentials(t *testing.T) {
	ctx := WithRelayOwner(context.Background(), 1)
	for _, owner := range []*int64{nil, relayTestID(2), relayTestID(1)} {
		account := &Account{ID: 8, Platform: PlatformQoder, Type: AccountTypeOAuth,
			OwnerUserID: owner, Credentials: map[string]any{"access_token": "private-token"}}
		s := qoderTestService()
		s.accountRepo = relayAccountRepoStub{account: account}
		_, authErr := s.Begin(ctx, 1, CreateAccountInput{}, account.ID)
		_, readErr := s.CurrentAccount(ctx, account, false)
		if owner == nil || *owner != 1 {
			require.Error(t, authErr)
			require.Error(t, readErr)
		} else {
			require.NoError(t, authErr)
			require.NoError(t, readErr)
		}
		s.Stop()
	}
}

func TestQoderRelayReauthorizationRechecksOwnershipAtCommit(t *testing.T) {
	s := qoderTestService()
	t.Cleanup(s.Stop)
	account := &Account{ID: 8, Platform: PlatformQoder, Type: AccountTypeOAuth, OwnerUserID: relayTestID(1)}
	s.accountRepo = relayAccountRepoStub{account: account}
	ctx := WithRelayOwner(context.Background(), 1)
	session, err := s.Begin(ctx, 1, CreateAccountInput{}, account.ID)
	require.NoError(t, err)
	_, err = s.Poll(ctx, 1, session.SessionID)
	require.NoError(t, err)
	account.OwnerUserID = relayTestID(2)
	_, err = s.Commit(ctx, 1, session.SessionID, nil)
	require.Error(t, err)
}

type qoderReauthRepoStub struct {
	relayAccountRepoStub
	saved map[string]any
}

func (r *qoderReauthRepoStub) ReauthorizeQoderOAuthCredentialsIfUnchanged(_ context.Context, _ int64, _ map[string]any, _ *int64, credentials map[string]any) (bool, error) {
	r.saved = credentials
	return true, nil
}

func TestQoderRelayReauthorizationPreservesConfiguredModelAliases(t *testing.T) {
	s := qoderTestService()
	t.Cleanup(s.Stop)
	mapping := map[string]any{"private-fast": "qwen3.8-flash"}
	account := &Account{ID: 8, Platform: PlatformQoder, Type: AccountTypeOAuth, OwnerUserID: relayTestID(1),
		Credentials: map[string]any{"access_token": "old-token", "model_mapping": mapping}}
	repo := &qoderReauthRepoStub{relayAccountRepoStub: relayAccountRepoStub{account: account}}
	s.accountRepo = repo
	ctx := WithRelayOwner(context.Background(), 1)
	session, err := s.Begin(ctx, 1, CreateAccountInput{}, account.ID)
	require.NoError(t, err)
	_, err = s.Poll(ctx, 1, session.SessionID)
	require.NoError(t, err)
	_, err = s.Commit(ctx, 1, session.SessionID, nil)
	require.NoError(t, err)
	require.Equal(t, mapping, repo.saved["model_mapping"])
	require.NotEqual(t, "old-token", repo.saved["access_token"])
}

func TestQoderRelayEditsPreserveMachineCredentialsAndRejectAPIKeyAccounts(t *testing.T) {
	existing := map[string]any{"access_token": "access", "refresh_token": "refresh", "machine_token": "machine-secret", "machine_id": "instance"}
	incoming := map[string]any{"machine_id": "instance", "model_mapping": map[string]any{"public-model": "qwen3.8-flash"}}
	require.NoError(t, ValidateRelayCredentials(AccountTypeOAuth, incoming))
	merged := MergePreservingSensitiveCreds(existing, incoming)
	require.Equal(t, "machine-secret", merged["machine_token"])
	require.Equal(t, "access", merged["access_token"])
	_, err := buildAccountForCreate(&CreateAccountInput{Platform: PlatformQoder, Type: AccountTypeAPIKey}, nil)
	require.Error(t, err)
}

func TestQoderRelayUsesExistingBillingAndCompositeRouting(t *testing.T) {
	account := &Account{ID: 8, Platform: PlatformQoder, Type: AccountTypeOAuth, OwnerUserID: relayTestID(1),
		Credentials: map[string]any{"model_mapping": map[string]any{"private-fast": "qwen3.8-flash"}}}
	private := &Group{ID: 9, Platform: PlatformComposite, OwnerUserID: relayTestID(1)}
	params := &postUsageBillingParams{User: &User{ID: 1}, APIKey: &APIKey{ID: 10, GroupID: &private.ID, Group: private},
		Account: account, Cost: &CostBreakdown{ActualCost: 1, TotalCost: 1}}
	privateBill := buildUsageBillingCommand("private-qoder", nil, params)
	require.Zero(t, privateBill.BalanceCost)
	require.Zero(t, privateBill.APIKeyQuotaCost)
	require.Nil(t, privateBill.SupplierUserID)
	public := &Group{ID: 11, Platform: PlatformQoder, SubscriptionType: SubscriptionTypeStandard, SupplierRate: 0.9}
	params.APIKey.GroupID, params.APIKey.Group = &public.ID, public
	params.Cost = &CostBreakdown{ActualCost: 1, TotalCost: 1}
	publicBill := buildUsageBillingCommand("shared-qoder", nil, params)
	require.Equal(t, 1.0, publicBill.BalanceCost)
	require.Equal(t, int64(1), *publicBill.SupplierUserID)
	require.Equal(t, 0.9, publicBill.SupplierRate)

	gateway := &GatewayService{accountRepo: &compositeOwnershipAccountRepo{accounts: []Account{*account}}}
	resolver := NewCompositeRouteResolver(nil)
	resolver.SetModelOwnershipResolver(gateway.resolveCompositeModelOwnership)
	decision, err := resolver.Resolve(context.Background(), private.ID, "private-fast", CompositeRouteEndpointMessages)
	require.NoError(t, err)
	require.True(t, decision.Matched)
	require.Equal(t, PlatformQoder, decision.TargetPlatform)
	require.Equal(t, PlatformQoder, QuotaPlatform(WithCompositeRouteDecision(context.Background(), decision), &APIKey{Group: private}))
}
