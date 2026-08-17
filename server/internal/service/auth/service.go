// Package auth implements HTTP-only administrator authentication.
package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/transport"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	errorsv1 "github.com/yvvlee/kirby/server/gen/kirby/errors/v1"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/config"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

const (
	loginAttemptLimit  = 5
	loginAttemptWindow = 15 * time.Minute
	refreshTokenTTL    = 7 * 24 * time.Hour
)

type UserRepository interface {
	FindByUsername(context.Context, string) (*model.User, error)
	GetByID(context.Context, int64) (*model.User, error)
	ListEnvironments(context.Context, *model.User) ([]model.Environment, error)
	UpdatePasswordHash(context.Context, int64, string, string) error
}

type RefreshTokenRepository interface {
	CreateSession(context.Context, *model.RefreshToken, time.Time) error
	Rotate(context.Context, []byte, *model.RefreshToken, time.Time) (*model.RefreshToken, error)
	RevokeSessionByTokenHash(context.Context, []byte, time.Time) error
}

type passwordHasher interface {
	Hash(string) (string, error)
	Verify(string, string) (bool, bool, error)
}

type accessTokenManager interface {
	Issue(int64, string) (string, time.Time, error)
	AccessTTL() time.Duration
}

// Service implements the generated AuthService HTTP contract.
type Service struct {
	users      UserRepository
	refresh    RefreshTokenRepository
	cache      cache.Store
	passwords  passwordHasher
	tokens     accessTokenManager
	origins    *authsession.OriginValidator
	refreshTTL time.Duration
	dummyHash  string
	now        func() time.Time
}

var _ adminv1.AuthServiceHTTPServer = (*Service)(nil)

// New constructs the production authentication service.
func New(cfg *config.Config, users UserRepository, refresh RefreshTokenRepository, cacheStore cache.Store) (*Service, error) {
	if cfg == nil {
		return nil, fmt.Errorf("authentication config is nil")
	}
	hasher, err := password.NewDefault()
	if err != nil {
		return nil, err
	}
	tokens, err := authjwt.New(cfg.JWT)
	if err != nil {
		return nil, err
	}
	origins, err := authsession.NewOriginValidator(cfg.Security.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	if cfg.JWT.RefreshTTL.Duration != refreshTokenTTL {
		return nil, fmt.Errorf("refresh TTL must be %s", refreshTokenTTL)
	}
	return newService(users, refresh, cacheStore, hasher, tokens, origins, cfg.JWT.RefreshTTL.Duration, time.Now)
}

func newService(
	users UserRepository,
	refresh RefreshTokenRepository,
	cacheStore cache.Store,
	hasher passwordHasher,
	tokens accessTokenManager,
	origins *authsession.OriginValidator,
	refreshTTL time.Duration,
	now func() time.Time,
) (*Service, error) {
	if users == nil || refresh == nil || cacheStore == nil || hasher == nil || tokens == nil || origins == nil || now == nil {
		return nil, fmt.Errorf("authentication dependencies are incomplete")
	}
	if refreshTTL <= tokens.AccessTTL() {
		return nil, fmt.Errorf("refresh TTL must be greater than access TTL")
	}
	dummyHash, err := hasher.Hash("kirby-dummy-password-that-is-never-valid")
	if err != nil {
		return nil, fmt.Errorf("create constant-work password hash: %w", err)
	}
	return &Service{
		users: users, refresh: refresh, cache: cacheStore, passwords: hasher, tokens: tokens,
		origins: origins, refreshTTL: refreshTTL, dummyHash: dummyHash, now: now,
	}, nil
}

func (s *Service) Login(ctx context.Context, request *adminv1.LoginRequest) (*adminv1.LoginReply, error) {
	if request == nil {
		return nil, errorsv1.ErrorBadRequest("HTTP authentication request required")
	}
	httpRequest, origin, originErr := s.validatedOriginRequest(ctx)
	if originErr != nil {
		return nil, originErr
	}
	if err := request.ValidateAll(); err != nil {
		return nil, authenticationFailed()
	}
	username := strings.TrimSpace(request.Username)
	if username == "" || request.Password == "" {
		return nil, authenticationFailed()
	}
	limitKey := loginRateKey(username, clientIP(httpRequest))
	attempts, err := s.cache.Increment(ctx, limitKey, loginAttemptWindow)
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	if attempts > loginAttemptLimit {
		return nil, errorsv1.ErrorTooManyRequests("too many authentication attempts")
	}

	user, findErr := s.users.FindByUsername(ctx, username)
	encodedHash := s.dummyHash
	if findErr == nil {
		encodedHash = user.PasswordHash
	} else if !errors.Is(findErr, repository.ErrUserNotFound) {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	matched, needsRehash, verifyErr := s.passwords.Verify(encodedHash, request.Password)
	if verifyErr != nil {
		if findErr == nil {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		return nil, authenticationFailed()
	}
	if findErr != nil || !matched || !user.Enabled {
		return nil, authenticationFailed()
	}
	if needsRehash {
		nextHash, err := s.passwords.Hash(request.Password)
		if err != nil {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		if err := s.users.UpdatePasswordHash(ctx, user.ID, user.PasswordHash, nextHash); err != nil {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		user.PasswordHash = nextHash
		user.Version++
	}

	now := s.now().UTC()
	plainRefresh, refreshHash, err := authsession.Generate()
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	refreshToken := &model.RefreshToken{
		UserID: user.ID, SessionID: uuid.NewString(), TokenHash: refreshHash, ExpiresAt: now.Add(s.refreshTTL),
	}
	accessToken, _, err := s.tokens.Issue(user.ID, refreshToken.SessionID)
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	if err := s.refresh.CreateSession(ctx, refreshToken, now); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, authenticationFailed()
		}
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	if err := s.cache.Delete(ctx, limitKey); err != nil {
		_ = s.refresh.RevokeSessionByTokenHash(ctx, refreshHash, now)
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	setCookie(ctx, authsession.Cookie(plainRefresh, refreshToken.ExpiresAt, authsession.SecureForOrigin(origin) || requestUsesHTTPS(httpRequest)))
	return loginReply(accessToken, s.tokens.AccessTTL(), user)
}

func (s *Service) Refresh(ctx context.Context, _ *emptypb.Empty) (*adminv1.LoginReply, error) {
	httpRequest, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	cookie, err := httpRequest.Cookie(authsession.CookieName)
	if err != nil || cookie.Value == "" {
		return nil, authenticationFailed()
	}
	now := s.now().UTC()
	plainNext, nextHash, err := authsession.Generate()
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	next := &model.RefreshToken{TokenHash: nextHash, ExpiresAt: now.Add(s.refreshTTL)}
	rotated, err := s.refresh.Rotate(ctx, authsession.Hash(cookie.Value), next, now)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenNotFound) || errors.Is(err, repository.ErrRefreshTokenExpired) || errors.Is(err, repository.ErrRefreshTokenReplay) {
			setCookie(ctx, authsession.ClearCookie(authsession.SecureForOrigin(origin)))
			return nil, authenticationFailed()
		}
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	user, err := s.users.GetByID(ctx, rotated.UserID)
	if err != nil || !user.Enabled {
		_ = s.refresh.RevokeSessionByTokenHash(ctx, rotated.TokenHash, now)
		setCookie(ctx, authsession.ClearCookie(authsession.SecureForOrigin(origin)))
		if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		return nil, authenticationFailed()
	}
	accessToken, _, err := s.tokens.Issue(user.ID, rotated.SessionID)
	if err != nil {
		_ = s.refresh.RevokeSessionByTokenHash(ctx, rotated.TokenHash, now)
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	setCookie(ctx, authsession.Cookie(plainNext, rotated.ExpiresAt, authsession.SecureForOrigin(origin)))
	return loginReply(accessToken, s.tokens.AccessTTL(), user)
}

func (s *Service) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	httpRequest, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	if cookie, cookieErr := httpRequest.Cookie(authsession.CookieName); cookieErr == nil && cookie.Value != "" {
		if err := s.refresh.RevokeSessionByTokenHash(ctx, authsession.Hash(cookie.Value), s.now().UTC()); err != nil {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
	}
	setCookie(ctx, authsession.ClearCookie(authsession.SecureForOrigin(origin)))
	return &emptypb.Empty{}, nil
}

func (s *Service) Me(ctx context.Context, _ *emptypb.Empty) (*adminv1.MeReply, error) {
	principal, ok := adminmiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, authenticationFailed()
	}
	user, err := s.users.GetByID(ctx, principal.UserID)
	if err != nil || !user.Enabled {
		if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		return nil, authenticationFailed()
	}
	environments, err := s.users.ListEnvironments(ctx, user)
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	protoUser, err := userToProto(user)
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	protoEnvironments := make([]*commonv1.Environment, 0, len(environments))
	for index := range environments {
		converted, err := environmentToProto(&environments[index])
		if err != nil {
			return nil, errorsv1.ErrorInternal("authentication is unavailable")
		}
		protoEnvironments = append(protoEnvironments, converted)
	}
	return &adminv1.MeReply{User: protoUser, Environments: protoEnvironments}, nil
}

func (s *Service) validatedOriginRequest(ctx context.Context) (*http.Request, string, error) {
	request, ok := kratoshttp.RequestFromServerContext(ctx)
	if !ok {
		return nil, "", errorsv1.ErrorBadRequest("HTTP authentication request required")
	}
	origin := request.Header.Get("Origin")
	if err := s.origins.Validate(origin); err != nil {
		return nil, "", errorsv1.ErrorForbidden("request origin is not allowed")
	}
	return request, origin, nil
}

func authenticationFailed() error {
	return errorsv1.ErrorUnauthorized("authentication failed")
}

func loginReply(accessToken string, ttl time.Duration, user *model.User) (*adminv1.LoginReply, error) {
	protoUser, err := userToProto(user)
	if err != nil {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	seconds := uint64(ttl / time.Second)
	if seconds == 0 || seconds > uint64(^uint32(0)) {
		return nil, errorsv1.ErrorInternal("authentication is unavailable")
	}
	return &adminv1.LoginReply{AccessToken: accessToken, ExpiresIn: uint32(seconds), User: protoUser}, nil
}

func userToProto(user *model.User) (*commonv1.User, error) {
	if user == nil || user.ID <= 0 || user.Version < 0 || uint64(user.Version) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("invalid user record")
	}
	return &commonv1.User{
		Id: user.ID, Username: user.Username, DisplayName: user.DisplayName, Enabled: user.Enabled,
		IsSystemAdmin: user.IsSystemAdmin, CreatedAt: formatTime(user.CreatedAt), UpdatedAt: formatTime(user.UpdatedAt), Version: uint32(user.Version),
	}, nil
}

func environmentToProto(environment *model.Environment) (*commonv1.Environment, error) {
	if environment == nil || environment.ID <= 0 || environment.Version < 0 || uint64(environment.Version) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("invalid environment record")
	}
	return &commonv1.Environment{
		Id: environment.ID, Key: environment.Key, Name: environment.Name, Description: environment.Description, Enabled: environment.Enabled,
		CreatedAt: formatTime(environment.CreatedAt), UpdatedAt: formatTime(environment.UpdatedAt), Version: uint32(environment.Version),
	}, nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func loginRateKey(username, ip string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username)) + "\x00" + ip))
	return "auth:login:" + fmt.Sprintf("%x", sum[:])
}

func clientIP(request *http.Request) string {
	if request == nil {
		return "unknown"
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return request.RemoteAddr
}

func requestUsesHTTPS(request *http.Request) bool {
	if request == nil {
		return false
	}
	if request.TLS != nil {
		return true
	}
	return authsession.SecureForOrigin(request.Header.Get("Origin"))
}

func setCookie(ctx context.Context, cookie *http.Cookie) {
	if cookie == nil {
		return
	}
	if serverTransport, ok := transport.FromServerContext(ctx); ok && serverTransport.ReplyHeader() != nil {
		serverTransport.ReplyHeader().Add("Set-Cookie", cookie.String())
	}
}
