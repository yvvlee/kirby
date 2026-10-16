package logic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

const (
	loginAttemptLimit  = 5
	loginAttemptWindow = 15 * time.Minute
	refreshTokenTTL    = 7 * 24 * time.Hour
)

var (
	ErrAuthFailed      = errors.New("authentication failed")
	ErrAuthUnavailable = errors.New("authentication is unavailable")
	ErrAuthRateLimited = errors.New("too many authentication attempts")
)

type AuthResult struct {
	AccessToken      string
	AccessTTL        time.Duration
	User             *model.User
	RefreshToken     string
	RefreshExpiresAt time.Time
	ClearRefresh     bool
}

type AuthProfile struct {
	User         *model.User
	Environments []model.Environment
}

type AuthLogic struct {
	users        repository.UserRepository
	refresh      repository.RefreshTokenRepository
	cache        cache.Store
	passwords    *password.Hasher
	tokens       *authjwt.Manager
	transactions TransactionRunner
	refreshTTL   time.Duration
	dummyHash    string
	now          func() time.Time
}

// NewAuthLogic constructs the AuthLogic with explicit application dependencies.
//
// Parameters: cfg supplies token lifetimes; users and refresh persist accounts and sessions; cacheStore enforces login limits; transactions makes session writes atomic; hasher verifies passwords; tokens signs access credentials.
// Returns: the authentication logic, or an invalid-lifetime or password-hasher initialization error.
// Throws: none; initialization errors are returned.
func NewAuthLogic(
	cfg *conf.Config,
	users repository.UserRepository,
	refresh repository.RefreshTokenRepository,
	cacheStore cache.Store,
	transactions TransactionRunner,
	hasher *password.Hasher,
	tokens *authjwt.Manager,
) (*AuthLogic, error) {
	if cfg.JWT.RefreshTTL.Duration != refreshTokenTTL {
		return nil, fmt.Errorf("refresh TTL must be %s", refreshTokenTTL)
	}
	if cfg.JWT.RefreshTTL.Duration <= tokens.AccessTTL() {
		return nil, fmt.Errorf("refresh TTL must be greater than access TTL")
	}
	dummyHash, err := hasher.Hash("kirby-dummy-password-that-is-never-valid")
	if err != nil {
		return nil, fmt.Errorf("create constant-work password hash: %w", err)
	}
	return &AuthLogic{
		users:        users,
		refresh:      refresh,
		cache:        cacheStore,
		passwords:    hasher,
		tokens:       tokens,
		transactions: transactions,
		refreshTTL:   cfg.JWT.RefreshTTL.Duration,
		dummyHash:    dummyHash,
		now:          time.Now,
	}, nil
}

// Login issues credentials only after password verification and session persistence succeed.
//
// Parameters: ctx controls authentication; username and plainPassword are login credentials; clientIP scopes the attempt limit.
// Returns: access and refresh credentials with the account, or ErrAuthFailed, ErrAuthRateLimited, or ErrAuthUnavailable with its cause.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Login(ctx context.Context, username, plainPassword, clientIP string) (*AuthResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || plainPassword == "" {
		return nil, ErrAuthFailed
	}
	limitKey := loginRateKey(username, clientIP)
	if err := l.checkLoginLimit(ctx, limitKey); err != nil {
		return nil, err
	}
	user, err := l.verifyLogin(ctx, username, plainPassword)
	if err != nil {
		return nil, err
	}
	result, err := l.createSession(ctx, user)
	if err != nil {
		return nil, err
	}
	if err := l.cache.Delete(ctx, limitKey); err != nil {
		revokeErr := l.revokeSession(ctx, authsession.Hash(result.RefreshToken), l.now().UTC())
		return nil, authUnavailable("clear login rate limit", errors.Join(err, revokeErr))
	}
	return result, nil
}

func (l *AuthLogic) checkLoginLimit(ctx context.Context, key string) error {
	attempts, err := l.cache.Increment(ctx, key, loginAttemptWindow)
	if err != nil {
		return authUnavailable("increment login rate limit", err)
	}
	if attempts > loginAttemptLimit {
		return ErrAuthRateLimited
	}
	return nil
}

func (l *AuthLogic) verifyLogin(ctx context.Context, username, plainPassword string) (*model.User, error) {
	user, findErr := l.users.FindByUsername(ctx, username)
	if findErr != nil && !errors.Is(findErr, repository.ErrUserNotFound) {
		return nil, authUnavailable("find login user", findErr)
	}
	encodedHash := l.dummyHash
	if findErr == nil {
		encodedHash = user.PasswordHash
	}
	matched, needsRehash, err := l.passwords.Verify(encodedHash, plainPassword)
	if err != nil {
		return nil, authUnavailable("verify login password", err)
	}
	if findErr != nil || !matched || !user.Enabled {
		return nil, ErrAuthFailed
	}
	if needsRehash {
		if err := l.rehashPassword(ctx, user, plainPassword); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (l *AuthLogic) rehashPassword(ctx context.Context, user *model.User, plainPassword string) error {
	nextHash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return authUnavailable("rehash login password", err)
	}
	if err := l.users.UpdatePasswordHash(ctx, user.ID, user.PasswordHash, nextHash); err != nil {
		return authUnavailable("save upgraded password hash", err)
	}
	user.PasswordHash = nextHash
	user.Version++
	return nil
}

func (l *AuthLogic) createSession(ctx context.Context, user *model.User) (*AuthResult, error) {
	now := l.now().UTC()
	plainRefresh, refreshHash, err := authsession.Generate()
	if err != nil {
		return nil, authUnavailable("generate login session", err)
	}
	refreshToken := &model.RefreshToken{
		UserID:    user.ID,
		SessionID: uuid.NewString(),
		TokenHash: refreshHash,
		ExpiresAt: now.Add(l.refreshTTL),
	}
	accessToken, _, err := l.tokens.Issue(user.ID, refreshToken.SessionID)
	if err != nil {
		return nil, authUnavailable("issue login access token", err)
	}
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.refresh.CreateSession(txctx, refreshToken, now)
	})
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrAuthFailed
	}
	if err != nil {
		return nil, authUnavailable("persist login session", err)
	}
	return &AuthResult{
		AccessToken:      accessToken,
		AccessTTL:        l.tokens.AccessTTL(),
		User:             user,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: refreshToken.ExpiresAt,
	}, nil
}

// Refresh rotates the refresh credential and treats failed mandatory revocation as unavailable authentication.
//
// Parameters: ctx controls authentication; refreshToken is the browser credential.
// Returns: replacement credentials, or an authentication error; ClearRefresh requests cookie removal after invalidation.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	now := l.now().UTC()
	plainNext, nextHash, err := authsession.Generate()
	if err != nil {
		return nil, authUnavailable("generate replacement session token", err)
	}
	next := &model.RefreshToken{
		TokenHash: nextHash,
		ExpiresAt: now.Add(l.refreshTTL),
	}
	rotated, err := l.rotateRefresh(ctx, authsession.Hash(refreshToken), next, now)
	if err != nil {
		return refreshFailure(err)
	}
	result, err := l.refreshedAccess(ctx, rotated, plainNext)
	if err == nil {
		return result, nil
	}
	if revokeErr := l.revokeSession(ctx, rotated.TokenHash, now); revokeErr != nil {
		return &AuthResult{ClearRefresh: true}, authUnavailable("revoke failed refresh session", errors.Join(err, revokeErr))
	}
	return &AuthResult{ClearRefresh: true}, err
}

func (l *AuthLogic) rotateRefresh(ctx context.Context, tokenHash []byte, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	var rotated *model.RefreshToken
	err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		rotated, err = l.refresh.Rotate(txctx, tokenHash, next, now)
		return err
	})
	if !errors.Is(err, repository.ErrRefreshTokenReplay) {
		return rotated, err
	}
	// The rejected rotation rolls back. Its replay revocation must commit separately.
	if revokeErr := l.revokeSession(ctx, tokenHash, now); revokeErr != nil {
		return nil, authUnavailable("revoke replayed session", errors.Join(err, revokeErr))
	}
	return nil, err
}

func refreshFailure(err error) (*AuthResult, error) {
	if errors.Is(err, ErrAuthUnavailable) {
		return &AuthResult{ClearRefresh: true}, err
	}
	if errors.Is(err, repository.ErrRefreshTokenNotFound) || errors.Is(err, repository.ErrRefreshTokenExpired) || errors.Is(err, repository.ErrRefreshTokenReplay) {
		return &AuthResult{ClearRefresh: true}, ErrAuthFailed
	}
	return nil, authUnavailable("rotate refresh session", err)
}

func (l *AuthLogic) refreshedAccess(ctx context.Context, token *model.RefreshToken, plainRefresh string) (*AuthResult, error) {
	user, err := l.activeUser(ctx, token.UserID)
	if err != nil {
		return nil, err
	}
	accessToken, _, err := l.tokens.Issue(user.ID, token.SessionID)
	if err != nil {
		return nil, authUnavailable("issue refreshed access token", err)
	}
	return &AuthResult{
		AccessToken:      accessToken,
		AccessTTL:        l.tokens.AccessTTL(),
		User:             user,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: token.ExpiresAt,
	}, nil
}

func (l *AuthLogic) revokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.refresh.RevokeSessionByTokenHash(txctx, tokenHash, now)
	})
}

// Logout revokes the entire browser refresh session in a transaction.
//
// Parameters: ctx controls revocation; refreshToken identifies the browser session.
// Returns: nil for a revoked or unknown session, or ErrAuthUnavailable retaining the revocation cause.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Logout(ctx context.Context, refreshToken string) error {
	if err := l.revokeSession(ctx, authsession.Hash(refreshToken), l.now().UTC()); err != nil {
		return authUnavailable("revoke logout session", err)
	}
	return nil
}

// Me rechecks the account and resolves its visible environments from persisted state.
//
// Parameters: ctx controls reads; userID is the authenticated token subject.
// Returns: the current account and environments, ErrAuthFailed, or ErrAuthUnavailable with its cause.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Me(ctx context.Context, userID int64) (*AuthProfile, error) {
	user, err := l.activeUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	environments, err := l.users.ListEnvironments(ctx, user)
	if err != nil {
		return nil, authUnavailable("list authenticated user environments", err)
	}
	return &AuthProfile{
		User:         user,
		Environments: environments,
	}, nil
}

func (l *AuthLogic) activeUser(ctx context.Context, userID int64) (*model.User, error) {
	user, err := l.users.GetByID(ctx, userID)
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrAuthFailed
	}
	if err != nil {
		return nil, authUnavailable("load authenticated user", err)
	}
	if !user.Enabled {
		return nil, ErrAuthFailed
	}
	return user, nil
}

func authUnavailable(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrAuthUnavailable, operation, err)
}

func loginRateKey(username, ip string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username)) + "\x00" + ip))
	return "auth:login:" + fmt.Sprintf("%x", sum[:])
}
