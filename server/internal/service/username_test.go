package service

import (
	"github.com/stretchr/testify/require"
	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"testing"
)

func TestUsernameCannotCollideWithEmailIdentifier(t *testing.T) {
	for _, username := range []string{"someone@example.com", " leading", "trailing ", "name with space", "@name"} {
		t.Run(username, func(t *testing.T) {
			require.Error(t, (&adminv1.RegisterRequest{Username: username, Nickname: "Test", Email: "test@example.com", Password: "long-password"}).Validate())
			require.Error(t, (&adminv1.CreateUserRequest{Username: username, Nickname: "Test", Email: "test@example.com", Password: "long-password"}).Validate())
			require.Error(t, (&adminv1.CreateInitialAdminRequest{Username: username, Nickname: "Test", Email: "test@example.com", Password: "long-password"}).Validate())
		})
	}
	require.NoError(t, (&adminv1.LoginRequest{Username: "test@example.com", Password: "long-password"}).Validate())
	for _, username := range []string{"alice", "User_1", "first.last", "service-account"} {
		require.NoError(t, (&adminv1.RegisterRequest{Username: username, Nickname: "Test", Email: "test@example.com", Password: "long-password"}).Validate())
	}
}
