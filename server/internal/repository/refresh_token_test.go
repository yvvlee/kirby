package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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

func TestRefreshReplayRevocationCommitsBeforeReturningError(t *testing.T) {
	engine, mock := newRepositoryMockEngine(t)
	now := time.Date(2026, 8, 17, 1, 2, 3, 0, time.UTC)
	replacedByID := int64(22)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT .* FROM ` + "`refresh_tokens`" + ` WHERE .*token_hash.*FOR UPDATE`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "session_id", "token_hash", "expires_at", "revoked_at", "replaced_by_id", "created_at", "updated_at",
		}).AddRow(21, 7, "session", make([]byte, 32), now.Add(time.Hour), now.Add(-time.Minute), replacedByID, now.Add(-time.Hour), now.Add(-time.Minute)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, ?), updated_at = ? WHERE user_id = ? AND session_id = ?")).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), int64(7), "session").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	repository, err := NewRefreshTokenRepository(engine)
	require.NoError(t, err)
	_, err = repository.Rotate(context.Background(), make([]byte, 32), &model.RefreshToken{
		TokenHash: make([]byte, 32), ExpiresAt: now.Add(7 * 24 * time.Hour),
	}, now)
	require.True(t, errors.Is(err, ErrRefreshTokenReplay), "Rotate error = %v", err)
	require.NoError(t, mock.ExpectationsWereMet())
}
