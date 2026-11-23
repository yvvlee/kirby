package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/auth/passwordpolicy"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/logic"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type authLogic interface {
	Login(ctx context.Context, username, plainPassword, clientIP string) (*logic.AuthResult, error)
	Register(ctx context.Context, username, nickname, email, password string) (*logic.AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (*logic.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (*logic.AuthProfile, error)
}

// Register creates an account only while public registration is enabled.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (s *AuthService) Register(ctx context.Context, request *adminv1.RegisterRequest) (*adminv1.LoginReply, error) {
	if err := passwordpolicy.Validate(request.Password); err != nil {
		return nil, errorsv1.ErrorBadRequest("%s", err.Error())
	}
	_, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.authLogic.Register(ctx, request.Username, request.Nickname, request.Email, request.Password)
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	setCookie(ctx, authsession.Cookie(result.RefreshToken, result.RefreshExpiresAt, authsession.SecureForOrigin(origin)))
	return loginReply(result.AccessToken, result.AccessTTL, result.User)
}

type AuthService struct {
	authLogic authLogic
	origins   *authsession.OriginValidator
	clientIPs *adminmiddleware.ClientIPResolver
}

var _ adminv1.AuthServiceHTTPServer = (*AuthService)(nil)

// NewAuthService wires authentication and the shared same-origin policy.
// Parameters: cfg supplies trusted proxies; authLogic manages authentication state.
// Returns: the authentication HTTP service.
// Throws: an error when the configured proxy networks are invalid.
func NewAuthService(cfg *conf.Config, authLogic *logic.AuthLogic) (*AuthService, error) {
	origins, err := authsession.NewOriginValidator(cfg.Security.TrustedProxies)
	if err != nil {
		return nil, err
	}
	clientIPs, err := adminmiddleware.NewClientIPResolver(cfg.Security.TrustedProxies)
	if err != nil {
		return nil, err
	}
	return &AuthService{
		authLogic: authLogic,
		origins:   origins,
		clientIPs: clientIPs,
	}, nil
}

// Login authenticates a browser request from the same public origin.
// Parameters: ctx contains the HTTP request; request contains credentials.
// Returns: the access token and account profile, with a refresh cookie.
// Throws: an API error for invalid origin, credentials, client address, or storage failure.
func (s *AuthService) Login(ctx context.Context, request *adminv1.LoginRequest) (*adminv1.LoginReply, error) {
	httpRequest, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	identifier := strings.TrimSpace(request.Username)
	if identifier == "" || request.Password == "" {
		return nil, authenticationFailed()
	}
	clientAddress, err := s.clientIPs.Resolve(httpRequest)
	if err != nil {
		return nil, errorsv1.ErrorBadRequest("client address is invalid")
	}
	result, err := s.authLogic.Login(ctx, identifier, request.Password, clientAddress.String())
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	setCookie(ctx, authsession.Cookie(result.RefreshToken, result.RefreshExpiresAt, authsession.SecureForOrigin(origin)))
	return loginReply(result.AccessToken, result.AccessTTL, result.User)
}

// Refresh rotates the refresh cookie after confirming the browser origin.
// Parameters: ctx contains the HTTP request and cookie; the request body is empty.
// Returns: a fresh access token and replacement refresh cookie.
// Throws: an API error for an invalid origin, refresh token, or storage failure.
func (s *AuthService) Refresh(ctx context.Context, _ *emptypb.Empty) (*adminv1.LoginReply, error) {
	httpRequest, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	cookie, err := httpRequest.Cookie(authsession.CookieName)
	if err != nil || cookie.Value == "" {
		return nil, authenticationFailed()
	}
	result, err := s.authLogic.Refresh(ctx, cookie.Value)
	if result != nil && result.ClearRefresh {
		setCookie(ctx, authsession.ClearCookie(authsession.SecureForOrigin(origin)))
	}
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	setCookie(ctx, authsession.Cookie(result.RefreshToken, result.RefreshExpiresAt, authsession.SecureForOrigin(origin)))
	return loginReply(result.AccessToken, result.AccessTTL, result.User)
}

// Logout revokes a same-origin browser session and clears its cookie.
// Parameters: ctx contains the HTTP request and cookie; the request body is empty.
// Returns: an empty response with the cleared cookie.
// Throws: an API error for an invalid origin or failed session revocation.
func (s *AuthService) Logout(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	httpRequest, origin, err := s.validatedOriginRequest(ctx)
	if err != nil {
		return nil, err
	}
	if cookie, cookieErr := httpRequest.Cookie(authsession.CookieName); cookieErr == nil && cookie.Value != "" {
		if err := s.authLogic.Logout(ctx, cookie.Value); err != nil {
			return nil, authAPIError(ctx, err)
		}
	}
	setCookie(ctx, authsession.ClearCookie(authsession.SecureForOrigin(origin)))
	return &emptypb.Empty{}, nil
}

// Me returns the authenticated account with its visible environments.
//
// Parameters: ctx carries the authenticated transport; the request body is empty.
// Returns: the public response, or a sanitized authorization or persistence error.
// Throws: none; failures are returned as errors.
func (s *AuthService) Me(ctx context.Context, _ *emptypb.Empty) (*adminv1.MeReply, error) {
	principal, ok := adminmiddleware.PrincipalFromContext(ctx)
	if !ok {
		return nil, authenticationFailed()
	}
	profile, err := s.authLogic.Me(ctx, principal.UserID)
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	return &adminv1.MeReply{
		User:         profile.User,
		Environments: profile.Environments,
	}, nil
}
func (s *AuthService) validatedOriginRequest(ctx context.Context) (*http.Request, string, error) {
	request, ok := kratoshttp.RequestFromServerContext(ctx)
	if !ok {
		return nil, "", errorsv1.ErrorBadRequest("HTTP authentication request required")
	}
	origin := request.Header.Get("Origin")
	if err := s.origins.Validate(request); err != nil {
		return nil, "", errorsv1.ErrorForbidden("request origin is not allowed")
	}
	return request, origin, nil
}

func authenticationFailed() error {
	return errorsv1.ErrorUnauthorized("authentication failed")
}

func loginReply(accessToken string, ttl time.Duration, user *commonv1.User) (*adminv1.LoginReply, error) {
	seconds := ttl / time.Second
	if seconds < 0 || seconds > math.MaxUint32 {
		return nil, errorsv1.ErrorInternal("token lifetime exceeds supported range")
	}
	return &adminv1.LoginReply{
		AccessToken: accessToken,
		ExpiresIn:   uint32(seconds),
		User:        user,
	}, nil
}

func setCookie(ctx context.Context, cookie *http.Cookie) {
	serverTransport, _ := transport.FromServerContext(ctx)
	serverTransport.ReplyHeader().Add("Set-Cookie", cookie.String())
}

func authAPIError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, repository.ErrKeyConflict):
		return errorsv1.ErrorConflict("username or email already exists")
	case errors.Is(err, logic.ErrRegistrationDisabled):
		return errorsv1.ErrorForbidden("self registration is disabled")
	case errors.Is(err, logic.ErrAuthFailed):
		return authenticationFailed()
	case errors.Is(err, logic.ErrAuthRateLimited):
		return errorsv1.ErrorTooManyRequests("too many authentication attempts")
	default:
		kratoslog.Context(ctx).Errorw("operation", "authenticate request", "error", err)
		return errorsv1.ErrorInternal("authentication is unavailable")
	}
}
