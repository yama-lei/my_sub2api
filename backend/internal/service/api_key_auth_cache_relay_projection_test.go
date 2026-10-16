package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyServiceRejectsV25AuthSnapshotMissingPrivateOwnerProjection(t *testing.T) {
	svc := &APIKeyService{}
	groupID := int64(12)
	key, used, err := svc.applyAuthCacheEntry("sk-stale-private-owner", &APIKeyAuthCacheEntry{
		Snapshot: &APIKeyAuthSnapshot{
			Version: 25, APIKeyID: 48, UserID: 8, GroupID: &groupID,
			User: APIKeyAuthUserSnapshot{ID: 8, Status: StatusActive},
			Group: &APIKeyAuthGroupSnapshot{
				ID: groupID, IsExclusive: true, Status: StatusActive,
			},
		},
	})
	require.NoError(t, err)
	require.False(t, used, "v25 snapshots may omit private ownership even when the database has it")
	require.Nil(t, key)
}

func TestAPIKeyAuthSnapshotPreservesPrivateOwnerAccess(t *testing.T) {
	svc := &APIKeyService{}
	ownerID, groupID := int64(8), int64(12)
	original := &APIKey{
		ID: 48, UserID: ownerID, GroupID: &groupID, Status: StatusActive,
		User: &User{ID: ownerID, Status: StatusActive},
		Group: &Group{
			ID: groupID, OwnerUserID: &ownerID, IsExclusive: true,
			Status: StatusActive, SupplierRate: 0.9,
		},
	}
	snapshot := svc.snapshotFromAPIKey(context.Background(), original)
	key, used, err := svc.applyAuthCacheEntry("sk-private-owner", &APIKeyAuthCacheEntry{Snapshot: snapshot})
	require.NoError(t, err)
	require.True(t, used)
	require.NotNil(t, key.Group)
	require.Equal(t, &ownerID, key.Group.OwnerUserID)
	require.InDelta(t, 0.9, key.Group.SupplierRate, 1e-9)
	require.True(t, key.Group.CanUse(key.User))
	require.False(t, key.Group.CanUse(&User{ID: ownerID + 1, AllowedGroups: []int64{groupID}}))
}
