package logic

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/converter"
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
	ErrRegistrationDisabled = errors.New("self registration is disabled")
	ErrAuthFailed           = errors.New("authentication failed")
	ErrAuthUnavailable      = errors.New("authentication is unavailable")
	ErrAuthRateLimited      = errors.New("too many authentication attempts")
)

type AuthResult struct {
	AccessToken      string
	AccessTTL        time.Duration
	User             *commonv1.User
	RefreshToken     string
	RefreshExpiresAt time.Time
	ClearRefresh     bool
}

type AuthProfile struct {
	User         *commonv1.User
	Environments []*commonv1.Environment
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
	settings     repository.ApplicationSettingsRepository
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
	settings repository.ApplicationSettingsRepository,
) (*AuthLogic, error) {
	if time.Duration(cfg.JWT.RefreshTTL)*time.Millisecond != refreshTokenTTL {
		return nil, fmt.Errorf("refresh TTL must be %s", refreshTokenTTL)
	}
	if time.Duration(cfg.JWT.RefreshTTL)*time.Millisecond <= tokens.AccessTTL() {
		return nil, fmt.Errorf("refresh TTL must be greater than access TTL")
	}
	dummyHash, err := hasher.Hash("kirby-dummy-password-that-is-never-valid")
	if err != nil {
		return nil, fmt.Errorf("create constant-work password hash: %w", err)
	}
	return &AuthLogic{
		users:        users,
		settings:     settings,
		refresh:      refresh,
		cache:        cacheStore,
		passwords:    hasher,
		tokens:       tokens,
		transactions: transactions,
		refreshTTL:   time.Duration(cfg.JWT.RefreshTTL) * time.Millisecond,
		dummyHash:    dummyHash,
	}, nil
}

// Register creates an account only while public registration is enabled.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (l *AuthLogic) Register(ctx context.Context, username, nickname, email, plainPassword string) (*AuthResult, error) {
	settings, err := l.settings.Get(ctx)
	if err != nil {
		return nil, authUnavailable("read registration settings", err)
	}
	if !settings.AllowSelfRegistration {
		return nil, ErrRegistrationDisabled
	}
	username = strings.ToLower(strings.TrimSpace(username))
	nickname = strings.TrimSpace(nickname)
	email = strings.ToLower(strings.TrimSpace(email))
	if username == "" || nickname == "" || email == "" || plainPassword == "" {
		return nil, ErrAuthFailed
	}
	hash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return nil, authUnavailable("hash registration password", err)
	}
	user := &model.User{
		Username:     username,
		Nickname:     nickname,
		Email:        email,
		PasswordHash: hash,
		Enabled:      true,
	}
	if err := l.users.CreateRegisteredUser(ctx, user); err != nil {
		return nil, err
	}
	return l.createSession(ctx, user)
}

// Login issues credentials only after password verification and session persistence succeed.
//
// Parameters: ctx controls authentication; username and plainPassword are login credentials; clientIP scopes the attempt limit.
// Returns: access and refresh credentials with the account, or ErrAuthFailed, ErrAuthRateLimited, or ErrAuthUnavailable with its cause.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Login(ctx context.Context, username, plainPassword, clientIP string) (*AuthResult, error) {
	username = strings.ToLower(strings.TrimSpace(username))
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
		revokeErr := l.revokeSession(ctx, authsession.Hash(result.RefreshToken), time.Now().UTC())
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
	user, findErr := l.users.FindByIdentifier(ctx, username)
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
	if _, err := l.users.UpdatePasswordHash(ctx, user.ID, user.PasswordHash, nextHash); err != nil {
		return authUnavailable("save upgraded password hash", err)
	}
	user.PasswordHash = nextHash
	user.Version++
	return nil
}

func (l *AuthLogic) createSession(ctx context.Context, user *model.User) (*AuthResult, error) {
	now := time.Now().UTC()
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
		_, err := l.refresh.CreateSession(txctx, refreshToken, now)
		return err
	})
	if errors.Is(err, repository.ErrUserNotFound) {
		return nil, ErrAuthFailed
	}
	if err != nil {
		return nil, authUnavailable("persist login session", err)
	}
	protoUser, err := converter.UserToProto(user)
	if err != nil {
		return nil, authUnavailable("convert login account", err)
	}
	return &AuthResult{
		AccessToken:      accessToken,
		AccessTTL:        l.tokens.AccessTTL(),
		User:             protoUser,
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
	now := time.Now().UTC()
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
	protoUser, err := converter.UserToProto(user)
	if err != nil {
		return nil, authUnavailable("convert refreshed account", err)
	}
	return &AuthResult{
		AccessToken:      accessToken,
		AccessTTL:        l.tokens.AccessTTL(),
		User:             protoUser,
		RefreshToken:     plainRefresh,
		RefreshExpiresAt: token.ExpiresAt,
	}, nil
}

func (l *AuthLogic) revokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		_, err := l.refresh.RevokeSessionByTokenHash(txctx, tokenHash, now)
		return err
	})
}

// Logout revokes the entire browser refresh session in a transaction.
//
// Parameters: ctx controls revocation; refreshToken identifies the browser session.
// Returns: nil for a revoked or unknown session, or ErrAuthUnavailable retaining the revocation cause.
// Throws: none; failures are returned as errors.
func (l *AuthLogic) Logout(ctx context.Context, refreshToken string) error {
	if err := l.revokeSession(ctx, authsession.Hash(refreshToken), time.Now().UTC()); err != nil {
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
	protoUser, err := converter.UserToProto(user)
	if err != nil {
		return nil, authUnavailable("convert authenticated account", err)
	}
	protoEnvironments := make([]*commonv1.Environment, 0, len(environments))
	for index := range environments {
		converted, err := converter.EnvironmentToProto(&environments[index])
		if err != nil {
			return nil, authUnavailable("convert authenticated environment", err)
		}
		protoEnvironments = append(protoEnvironments, converted)
	}
	return &AuthProfile{
		User:         protoUser,
		Environments: protoEnvironments,
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
