package service

import (
	"context"
	"errors"
	"fmt"
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
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
)

type authLogic interface {
	Login(ctx context.Context, username, plainPassword, clientIP string) (*logic.AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (*logic.AuthResult, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (*logic.AuthProfile, error)
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
	username := strings.TrimSpace(request.Username)
	if username == "" || request.Password == "" {
		return nil, authenticationFailed()
	}
	clientAddress, err := s.clientIPs.Resolve(httpRequest)
	if err != nil {
		return nil, errorsv1.ErrorBadRequest("client address is invalid")
	}
	result, err := s.authLogic.Login(ctx, username, request.Password, clientAddress.String())
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	setCookie(ctx, authsession.Cookie(result.RefreshToken, result.RefreshExpiresAt, authsession.SecureForOrigin(origin)))
	return loginReply(ctx, result.AccessToken, result.AccessTTL, result.User)
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
	return loginReply(ctx, result.AccessToken, result.AccessTTL, result.User)
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

// Me uses the shared account and environment mappings for the authenticated profile.
//
// Parameters: ctx carries the authenticated transport; the request body is empty.
// Returns: the public response, or a sanitized authorization, conflict, conversion, or persistence error.
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
	user, environments := profile.User, profile.Environments
	protoUser, err := converter.UserToProto(user)
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	protoEnvironments := make([]*commonv1.Environment, 0, len(environments))
	for index := range environments {
		converted, err := converter.EnvironmentToProto(&environments[index])
		if err != nil {
			return nil, authAPIError(ctx, err)
		}
		protoEnvironments = append(protoEnvironments, converted)
	}
	return &adminv1.MeReply{
		User:         protoUser,
		Environments: protoEnvironments,
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

func loginReply(ctx context.Context, accessToken string, ttl time.Duration, user *model.User) (*adminv1.LoginReply, error) {
	protoUser, err := converter.UserToProto(user)
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	seconds, err := safeint.Uint32FromInt64(int64(ttl / time.Second))
	if err != nil {
		return nil, authAPIError(ctx, err)
	}
	if seconds == 0 {
		return nil, authAPIError(ctx, fmt.Errorf("access token lifetime must be positive"))
	}
	return &adminv1.LoginReply{
		AccessToken: accessToken,
		ExpiresIn:   seconds,
		User:        protoUser,
	}, nil
}

func setCookie(ctx context.Context, cookie *http.Cookie) {
	serverTransport, _ := transport.FromServerContext(ctx)
	serverTransport.ReplyHeader().Add("Set-Cookie", cookie.String())
}

func authAPIError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, logic.ErrAuthFailed):
		return authenticationFailed()
	case errors.Is(err, logic.ErrAuthRateLimited):
		return errorsv1.ErrorTooManyRequests("too many authentication attempts")
	default:
		kratoslog.Context(ctx).Errorw("operation", "authenticate request", "error", err)
		return errorsv1.ErrorInternal("authentication is unavailable")
	}
}
