package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type authTestUsers struct {
	repository.UserRepository
	user                   *model.User
	previousHash, nextHash string
}

func (r *authTestUsers) FindByUsername(context.Context, string) (*model.User, error) {
	user := *r.user
	return &user, nil
}

func (r *authTestUsers) UpdatePasswordHash(_ context.Context, _ int64, previous, next string) error {
	r.previousHash, r.nextHash = previous, next
	return nil
}

type authTestRefreshTokens struct {
	repository.RefreshTokenRepository
	created     *model.RefreshToken
	rotateErr   error
	revokeErr   error
	revokedHash []byte
	revokedAt   time.Time
}

func (r *authTestRefreshTokens) CreateSession(_ context.Context, token *model.RefreshToken, _ time.Time) error {
	r.created = token
	return nil
}

func (r *authTestRefreshTokens) Rotate(context.Context, []byte, *model.RefreshToken, time.Time) (*model.RefreshToken, error) {
	return nil, r.rotateErr
}

func (r *authTestRefreshTokens) RevokeSessionByTokenHash(_ context.Context, hash []byte, now time.Time) error {
	r.revokedHash, r.revokedAt = hash, now
	return r.revokeErr
}

func newAuthTestLogic(t *testing.T, users *authTestUsers, refresh *authTestRefreshTokens) *AuthLogic {
	t.Helper()
	hasher, err := password.New(password.Params{
		MemoryKiB:   16 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	require.NoError(t, err)
	cfg := &conf.Config{
		JWT: conf.JWTConfig{
			Issuer:    "auth-test",
			AccessTTL: conf.Duration{Duration: 15 * time.Minute},
			RefreshTTL: conf.Duration{
				Duration: 7 * 24 * time.Hour,
			},
		},
	}
	tokens, err := authjwt.New(authjwt.Config{
		Issuer:    cfg.JWT.Issuer,
		ActiveKID: "primary",
		AccessTTL: cfg.JWT.AccessTTL.Duration,
		Keys: map[string]conf.Secret{
			"primary": conf.NewSecret("01234567890123456789012345678901"),
		},
	})
	require.NoError(t, err)
	logic, err := NewAuthLogic(cfg, users, refresh, cache.NewMemory(), directTransactions{}, hasher, tokens)
	require.NoError(t, err)
	return logic
}

func TestAuthLoginRehashesPasswordAndUsesRefreshClock(t *testing.T) {
	oldHasher, err := password.New(password.Params{
		MemoryKiB:   8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	require.NoError(t, err)
	oldHash, err := oldHasher.Hash("correct password")
	require.NoError(t, err)
	users := &authTestUsers{
		user: &model.User{
			Meta: model.Meta{
				ID:      1,
				Version: 2,
			},
			Username:     "alice",
			PasswordHash: oldHash,
			Enabled:      true,
		},
	}
	refresh := &authTestRefreshTokens{}
	logic := newAuthTestLogic(t, users, refresh)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	logic.now = func() time.Time { return now }
	result, err := logic.Login(context.Background(), " alice ", "correct password", "192.0.2.1")
	require.NoError(t, err)
	require.Equal(t, oldHash, users.previousHash)
	require.NotEqual(t, oldHash, users.nextHash)
	matched, rehash, err := logic.passwords.Verify(users.nextHash, "correct password")
	require.NoError(t, err)
	require.True(t, matched)
	require.False(t, rehash)
	require.Equal(t, int64(3), result.User.Version)
	require.Equal(t, now.UTC().Add(7*24*time.Hour), result.RefreshExpiresAt)
	require.Equal(t, 15*time.Minute, result.AccessTTL)
	require.Equal(t, authsession.Hash(result.RefreshToken), refresh.created.TokenHash)
	require.Equal(t, result.RefreshExpiresAt, refresh.created.ExpiresAt)
}

func TestAuthRefreshReplayRevokesSessionAndClearsCredential(t *testing.T) {
	refresh := &authTestRefreshTokens{rotateErr: repository.ErrRefreshTokenReplay}
	logic := newAuthTestLogic(t, &authTestUsers{}, refresh)
	now := time.Date(2026, 10, 9, 4, 0, 0, 0, time.UTC)
	logic.now = func() time.Time { return now }
	result, err := logic.Refresh(context.Background(), "replayed-refresh-token")
	require.ErrorIs(t, err, ErrAuthFailed)
	require.True(t, result.ClearRefresh)
	require.Equal(t, authsession.Hash("replayed-refresh-token"), refresh.revokedHash)
	require.Equal(t, now, refresh.revokedAt)
}

type authFailingCache struct {
	cache.Store
	failure error
}

func (c authFailingCache) Increment(context.Context, string, time.Duration) (int64, error) {
	return 0, c.failure
}

func TestAuthLoginPreservesRateLimitStorageCause(t *testing.T) {
	failure := errors.New("rate limit storage unavailable")
	logic := newAuthTestLogic(t, &authTestUsers{}, &authTestRefreshTokens{})
	logic.cache = authFailingCache{failure: failure}
	_, err := logic.Login(context.Background(), "alice", "plain password", "192.0.2.1")
	require.ErrorIs(t, err, ErrAuthUnavailable)
	require.ErrorIs(t, err, failure)
	require.NotContains(t, err.Error(), "plain password")
}

func TestAuthRefreshReplayRevocationFailureIsUnavailable(t *testing.T) {
	failure := errors.New("session revocation failed")
	refresh := &authTestRefreshTokens{
		rotateErr: repository.ErrRefreshTokenReplay,
		revokeErr: failure,
	}
	logic := newAuthTestLogic(t, &authTestUsers{}, refresh)
	result, err := logic.Refresh(context.Background(), "replayed-credential")
	require.ErrorIs(t, err, ErrAuthUnavailable)
	require.ErrorIs(t, err, failure)
	require.ErrorIs(t, err, repository.ErrRefreshTokenReplay)
	require.NotErrorIs(t, err, ErrAuthFailed)
	require.True(t, result.ClearRefresh)
	require.NotContains(t, err.Error(), "replayed-credential")
}
