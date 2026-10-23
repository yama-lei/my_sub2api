package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRelayGroupStatusHidesOtherSuppliersDetails(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Minute)
	g := &Group{ID: 5, Name: "Public pool", Platform: PlatformOpenAI, SupplierRate: 0.9, Status: StatusActive}
	accounts := []Account{
		{ID: 1, Name: "mine", OwnerUserID: relayTestID(7), Platform: PlatformOpenAI, Type: AccountTypeOAuth,
			Status: StatusError, ErrorMessage: "token revoked"},
		{ID: 2, Name: "someone@example.com", OwnerUserID: relayTestID(8), Platform: PlatformOpenAI, Type: AccountTypeOAuth,
			Status: StatusActive, Schedulable: true, RateLimitResetAt: &later, ErrorMessage: "org-secret", TempUnschedulableReason: "x"},
		{ID: 3, Name: "platform", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
	}
	got := buildRelayGroupStatus(g, accounts, 7, now)
	require.Equal(t, RelayGroupPublic, got.Kind)
	require.Equal(t, 3, got.Total)
	require.Equal(t, 1, got.Available)
	require.Equal(t, 1, got.Mine)
	require.Equal(t, RelayAccountStatus{Mine: true, ID: 1, Name: "mine", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		State: RelayAccountError, ErrorMessage: "token revoked"}, got.Accounts[0])
	require.Equal(t, RelayAccountStatus{Platform: PlatformOpenAI, Type: AccountTypeOAuth, State: RelayAccountRateLimited, Until: &later}, got.Accounts[1])
	require.Equal(t, RelayAccountStatus{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, State: RelayAccountAvailable}, got.Accounts[2])
}

func TestRelayAccountStatusReportsFirstBlockingReason(t *testing.T) {
	now := time.Now()
	past, later := now.Add(-time.Minute), now.Add(time.Minute)
	cases := map[string]Account{
		RelayAccountDisabled:   {Status: StatusDisabled},
		RelayAccountPaused:     {Status: StatusActive},
		RelayAccountExpired:    {Status: StatusActive, Schedulable: true, AutoPauseOnExpired: true, ExpiresAt: &past, OverloadUntil: &later},
		RelayAccountOverloaded: {Status: StatusActive, Schedulable: true, OverloadUntil: &later, RateLimitResetAt: &later},
		RelayAccountCooling:    {Status: StatusActive, Schedulable: true, RateLimitResetAt: &past, TempUnschedulableUntil: &later},
		RelayAccountAvailable:  {Status: StatusActive, Schedulable: true, RateLimitResetAt: &past},
	}
	for want, a := range cases {
		require.Equal(t, want, relayAccountStatus(&a, now).State, want)
	}
}

func TestRelayGroupKind(t *testing.T) {
	require.Equal(t, RelayGroupPrivate, relayGroupKind(&Group{OwnerUserID: relayTestID(1), IsExclusive: true}))
	require.Equal(t, RelayGroupPublic, relayGroupKind(&Group{SubscriptionType: SubscriptionTypeStandard}))
	require.Equal(t, RelayGroupStandard, relayGroupKind(&Group{IsExclusive: true}))
	require.Equal(t, RelayGroupStandard, relayGroupKind(&Group{SubscriptionType: SubscriptionTypeSubscription}))
}
