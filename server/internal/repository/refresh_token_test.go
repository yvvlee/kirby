package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestNewRefreshTokenValidationRejectsPlaintextSizedData(t *testing.T) {
	token := &model.RefreshToken{
		UserID:    1,
		SessionID: "session",
		TokenHash: make([]byte, 43),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := validateNewRefreshToken(token); err == nil {
		t.Fatal("non-SHA-256 token value was accepted")
	}
	token.TokenHash = make([]byte, 32)
	if err := validateNewRefreshToken(token); err != nil {
		t.Fatalf("valid refresh record rejected: %v", err)
	}
}

func TestValidateNewRefreshTokenRejectsRevokedRecords(t *testing.T) {
	now := time.Now()
	revoked := now.Add(-time.Minute)
	token := &model.RefreshToken{
		UserID:    1,
		SessionID: "session",
		TokenHash: make([]byte, 32),
		ExpiresAt: now.Add(time.Hour),
	}
	token.RevokedAt = &revoked
	require.Error(t, validateNewRefreshToken(token))
	token.RevokedAt = nil
	token.ReplacedByID = new(int64)
	require.Error(t, validateNewRefreshToken(token))
}
