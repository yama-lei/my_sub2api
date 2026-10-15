package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetByKeyForAuthPreservesRelayOwnershipAndSupplierRate(t *testing.T) {
	for _, tc := range []struct {
		name         string
		private      bool
		supplierRate float64
	}{
		{name: "private owner", private: true},
		{name: "public supplier rate", supplierRate: 0.9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, client := newAPIKeyRepoSQLite(t)
			ctx := context.Background()
			owner := mustCreateAPIKeyRepoUser(t, ctx, client, "relay-auth@example.test")
			var ownerID *int64
			if tc.private {
				ownerID = &owner.ID
			}
			group, err := client.Group.Create().
				SetName(tc.name).
				SetPlatform(service.PlatformAnthropic).
				SetStatus(service.StatusActive).
				SetSubscriptionType(service.SubscriptionTypeStandard).
				SetIsExclusive(tc.private).
				SetNillableOwnerUserID(ownerID).
				SetSupplierRate(tc.supplierRate).
				Save(ctx)
			require.NoError(t, err)
			key := &service.APIKey{
				UserID: owner.ID, Key: "sk-relay-auth-projection", Name: tc.name,
				GroupID: &group.ID, Status: service.StatusActive,
			}
			require.NoError(t, repo.Create(ctx, key))

			got, err := repo.GetByKeyForAuth(ctx, key.Key)
			require.NoError(t, err)
			require.NotNil(t, got.Group)
			require.Equal(t, tc.private, got.Group.IsPrivate())
			require.Equal(t, ownerID, got.Group.OwnerUserID)
			require.InDelta(t, tc.supplierRate, got.Group.SupplierRate, 1e-9)
			if tc.private {
				require.Empty(t, got.User.AllowedGroups)
				require.True(t, got.Group.CanUse(got.User), "owners need no exclusive-group allowlist entry")
				require.False(t, got.Group.CanUse(&service.User{
					ID: owner.ID + 1, AllowedGroups: []int64{group.ID},
				}), "an allowlist entry must not grant access to another user's private group")
			}
		})
	}
}
