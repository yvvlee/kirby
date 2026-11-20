package repository

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyManagementIncludesListingWithoutReadGrant(t *testing.T) {
	keys := permissionKeysForPoint("API_KEY_MANAGE")
	require.Contains(t, keys, "project:api_key:read")
	require.Contains(t, keys, "project:api_key:manage")
	require.NotContains(t, keys, "config:read")
}

func TestSnapshotTransferPermissionsFollowReadAndWrite(t *testing.T) {
	require.Contains(t, permissionKeysForPoint("READ"), "snapshot:export")
	require.NotContains(t, permissionKeysForPoint("READ"), "snapshot:import")
	require.Contains(t, permissionKeysForPoint("WRITE"), "snapshot:import")
	require.NotContains(t, permissionKeysForPoint("WRITE"), "snapshot:publish")
	require.Equal(t, []string{"snapshot:publish"}, permissionKeysForPoint("PUBLISH"))
}
