package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/middleware"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type fakeUsers struct {
	repository.UserRepository
	mu           sync.Mutex
	byName       map[string]*model.User
	environments []model.Environment
}

func (f *fakeUsers) FindByUsername(_ context.Context, username string) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byName[username]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	copy := *user
	return &copy, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id int64) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, user := range f.byName {
		if user.ID == id {
			copy := *user
			return &copy, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeUsers) ListEnvironments(context.Context, *model.User) ([]model.Environment, error) {
	return append([]model.Environment(nil), f.environments...), nil
}

func (f *fakeUsers) UpdatePasswordHash(_ context.Context, id int64, previousHash, nextHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, user := range f.byName {
		if user.ID == id && user.PasswordHash == previousHash {
			user.PasswordHash = nextHash
			return nil
		}
	}
	return repository.ErrUserVersionConflict
}

type fakeRefreshTokens struct {
	repository.RefreshTokenRepository
	mu      sync.Mutex
	records map[string]*model.RefreshToken
	replays int
}

func newFakeRefreshTokens() *fakeRefreshTokens {
	return &fakeRefreshTokens{records: make(map[string]*model.RefreshToken)}
}

func hashKey(hash []byte) string { return string(hash) }

func (f *fakeRefreshTokens) CreateSession(_ context.Context, token *model.RefreshToken, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := *token
	copy.ID = int64(len(f.records) + 1)
	token.ID = copy.ID
	f.records[hashKey(token.TokenHash)] = &copy
	return nil
}

func (f *fakeRefreshTokens) Rotate(_ context.Context, hash []byte, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	current, ok := f.records[hashKey(hash)]
	if !ok {
		return nil, repository.ErrRefreshTokenNotFound
	}
	if current.RevokedAt != nil {
		if current.ReplacedByID != nil {
			f.replays++
			for _, token := range f.records {
				if token.UserID == current.UserID && token.SessionID == current.SessionID && token.RevokedAt == nil {
					revoked := now
					token.RevokedAt = &revoked
				}
			}
			return nil, repository.ErrRefreshTokenReplay
		}
		return nil, repository.ErrRefreshTokenNotFound
	}
	if !now.Before(current.ExpiresAt) {
		return nil, repository.ErrRefreshTokenExpired
	}
	next.ID = int64(len(f.records) + 1)
	next.UserID = current.UserID
	next.SessionID = current.SessionID
	revoked := now
	current.RevokedAt = &revoked
	current.ReplacedByID = &next.ID
	copy := *next
	f.records[hashKey(next.TokenHash)] = &copy
	return next, nil
}

func (f *fakeRefreshTokens) RevokeSessionByTokenHash(_ context.Context, hash []byte, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	current, ok := f.records[hashKey(hash)]
	if !ok {
		return nil
	}
	for _, token := range f.records {
		if token.UserID == current.UserID && token.SessionID == current.SessionID && token.RevokedAt == nil {
			revoked := now
			token.RevokedAt = &revoked
		}
	}
	return nil
}

func (f *fakeRefreshTokens) activeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, token := range f.records {
		if token.RevokedAt == nil {
			count++
		}
	}
	return count
}

type httpHeader http.Header

func (h httpHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h httpHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h httpHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h httpHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h httpHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}

type httpTransport struct {
	request *http.Request
	reply   httpHeader
}

func (*httpTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (*httpTransport) Endpoint() string                  { return "http://localhost" }
func (*httpTransport) Operation() string                 { return "auth-test" }
func (t *httpTransport) RequestHeader() transport.Header { return httpHeader(t.request.Header) }
func (t *httpTransport) ReplyHeader() transport.Header   { return t.reply }
func (t *httpTransport) Request() *http.Request          { return t.request }
func (*httpTransport) PathTemplate() string              { return "/auth/test" }

func testService(t *testing.T) (*AuthService, *fakeUsers, *fakeRefreshTokens) {
	return testServiceWithTrustedProxies(t, nil)
}

func testServiceWithTrustedProxies(t *testing.T, trustedProxies []string) (*AuthService, *fakeUsers, *fakeRefreshTokens) {
	t.Helper()
	hasher, err := password.New(password.Params{
		MemoryKiB:   8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := hasher.Hash("correct password")
	if err != nil {
		t.Fatal(err)
	}
	users := &fakeUsers{byName: map[string]*model.User{
		"alice": {
			Meta: model.Meta{
				ID:      1,
				Version: 1,
			},
			Username:     "alice",
			DisplayName:  "Alice",
			PasswordHash: hash,
			Enabled:      true,
		},
	}}
	refresh := newFakeRefreshTokens()
	cfg := &conf.Config{
		JWT: conf.JWTConfig{
			Issuer:     "auth-test",
			AccessTTL:  conf.Duration{Duration: 15 * time.Minute},
			RefreshTTL: conf.Duration{Duration: 7 * 24 * time.Hour},
		},
		Security: conf.SecurityConfig{TrustedProxies: trustedProxies},
	}
	tokens, err := authjwt.New(authjwt.Config{
		Issuer:    "auth-test",
		ActiveKID: "primary",
		AccessTTL: 15 * time.Minute,
		Keys:      map[string]conf.Secret{"primary": conf.NewSecret("01234567890123456789012345678901")},
	})
	if err != nil {
		t.Fatal(err)
	}
	authLogic, err := logic.NewAuthLogic(cfg, users, refresh, cache.NewMemory(), serviceTransactions{}, hasher, tokens)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAuthService(cfg, authLogic)
	if err != nil {
		t.Fatal(err)
	}
	return service, users, refresh
}

func requestContext(method, origin, cookie string) (context.Context, *httpTransport) {
	request, _ := http.NewRequest(method, "https://kirby.example.com/auth/test", bytes.NewReader(nil))
	request.RemoteAddr = "192.0.2.1:12345"
	request.TLS = &tls.ConnectionState{}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	t := &httpTransport{request: request, reply: httpHeader{}}
	return transport.NewServerContext(context.Background(), t), t
}

func responseCookie(t *testing.T, tr *httpTransport) *http.Cookie {
	t.Helper()
	response := &http.Response{Header: http.Header{"Set-Cookie": tr.reply.Values("Set-Cookie")}}
	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("Set-Cookie count = %d, headers=%v", len(cookies), tr.reply)
	}
	return cookies[0]
}

func TestLoginFailureIsUniformAndRateLimited(t *testing.T) {
	service, _, _ := testService(t)
	ctx, _ := requestContext(http.MethodPost, "https://kirby.example.com", "")
	wrong := &adminv1.LoginRequest{
		Username: "alice",
		Password: "wrong",
	}
	unknown := &adminv1.LoginRequest{
		Username: "missing",
		Password: "wrong",
	}
	for _, request := range []*adminv1.LoginRequest{wrong, unknown} {
		_, err := service.Login(ctx, request)
		kratosErr := kratoserrors.FromError(err)
		if kratosErr.Code != http.StatusUnauthorized || kratosErr.Message != "authentication failed" {
			t.Fatalf("non-uniform login failure: %+v", kratosErr)
		}
	}

	for attempt := 2; attempt <= 5; attempt++ {
		_, _ = service.Login(ctx, wrong)
	}
	_, err := service.Login(ctx, wrong)
	if got := kratoserrors.FromError(err).Code; got != http.StatusTooManyRequests {
		t.Fatalf("rate-limited login status = %d", got)
	}
}

func TestLoginRateLimitIgnoresForgedForwardedHeader(t *testing.T) {
	service, _, _ := testServiceWithTrustedProxies(t, []string{})
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	request := &adminv1.LoginRequest{
		Username: "alice",
		Password: "wrong",
	}
	for attempt := 1; attempt <= 5; attempt++ {
		requestTransport.request.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", attempt))
		_, err := service.Login(ctx, request)
		if code := kratoserrors.FromError(err).Code; code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, code)
		}
	}
	requestTransport.request.Header.Set("X-Forwarded-For", "203.0.113.200")
	_, err := service.Login(ctx, request)
	if code := kratoserrors.FromError(err).Code; code != http.StatusTooManyRequests {
		t.Fatalf("forged header bypassed rate limit: status=%d", code)
	}
}

func TestLoginRateLimitUsesResolvedClientBehindTrustedProxy(t *testing.T) {
	service, _, _ := testServiceWithTrustedProxies(t, []string{"10.0.0.0/8"})
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requestTransport.request.RemoteAddr = "10.0.0.2:443"
	request := &adminv1.LoginRequest{
		Username: "alice",
		Password: "wrong",
	}
	requestTransport.request.Header.Set("X-Forwarded-For", "198.51.100.10")
	for attempt := 1; attempt <= 5; attempt++ {
		_, err := service.Login(ctx, request)
		if code := kratoserrors.FromError(err).Code; code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, code)
		}
	}

	requestTransport.request.Header.Set("X-Forwarded-For", "198.51.100.11")
	_, err := service.Login(ctx, request)
	if code := kratoserrors.FromError(err).Code; code != http.StatusUnauthorized {
		t.Fatalf("independent proxy client status = %d", code)
	}
}

func TestLoginWithoutForwardedClientAddressUsesProxyAddress(t *testing.T) {
	service, _, _ := testServiceWithTrustedProxies(t, []string{"10.0.0.0/8"})
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requestTransport.request.RemoteAddr = "10.0.0.2:443"
	if _, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	}); err != nil {
		t.Fatalf("proxy without X-Forwarded-For should remain usable: %v", err)
	}
}

func TestLoginRejectsInvalidForwardedClientAddress(t *testing.T) {
	service, _, _ := testServiceWithTrustedProxies(t, []string{"10.0.0.0/8"})
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requestTransport.request.RemoteAddr = "10.0.0.2:443"
	requestTransport.request.Header.Set("X-Forwarded-For", "not-an-ip")
	_, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	})
	kratosErr := kratoserrors.FromError(err)
	if kratosErr.Code != http.StatusBadRequest || kratosErr.Message != "client address is invalid" {
		t.Fatalf("unexpected proxy header error: %+v", kratosErr)
	}
}

func TestLoginValidatesRequestBeforePasswordWork(t *testing.T) {
	service, _, refresh := testService(t)
	ctx, _ := requestContext(http.MethodPost, "https://kirby.example.com", "")
	_, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: strings.Repeat("u", 129),
		Password: "wrong",
	})
	kratosErr := kratoserrors.FromError(err)
	if kratosErr.Code != http.StatusUnauthorized || kratosErr.Message != "authentication failed" {
		t.Fatalf("invalid login request leaked validation details: %+v", kratosErr)
	}
	if len(refresh.records) != 0 {
		t.Fatal("invalid login request created a session")
	}
}

func TestRefreshRotatesAndReplayRevokesSession(t *testing.T) {
	service, _, refresh := testService(t)
	loginContext, loginTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	if _, err := service.Login(loginContext, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	}); err != nil {
		t.Fatal(err)
	}
	firstCookie := responseCookie(t, loginTransport)

	refreshContext, refreshTransport := requestContext(http.MethodPost, "https://kirby.example.com", firstCookie.String())
	if _, err := service.Refresh(refreshContext, &emptypb.Empty{}); err != nil {
		t.Fatal(err)
	}
	secondCookie := responseCookie(t, refreshTransport)
	if secondCookie.Value == firstCookie.Value || refresh.activeCount() != 1 {
		t.Fatalf("refresh did not rotate: first=%q second=%q active=%d", firstCookie.Value, secondCookie.Value, refresh.activeCount())
	}

	replayContext, _ := requestContext(http.MethodPost, "https://kirby.example.com", firstCookie.String())
	_, err := service.Refresh(replayContext, &emptypb.Empty{})
	if got := kratoserrors.FromError(err).Code; got != http.StatusUnauthorized {
		t.Fatalf("refresh replay status = %d", got)
	}
	if refresh.replays != 1 || refresh.activeCount() != 0 {
		t.Fatalf("replay did not revoke session: replays=%d active=%d", refresh.replays, refresh.activeCount())
	}
}

func TestRefreshAndLogoutRequireExactOrigin(t *testing.T) {
	service, _, _ := testService(t)
	for _, origin := range []string{"", "null", "https://kirby.example.com.evil", "http://kirby.example.com", "https://kirby.example.com:8443"} {
		ctx, _ := requestContext(http.MethodPost, origin, authsession.CookieName+"=opaque")
		_, loginErr := service.Login(ctx, &adminv1.LoginRequest{
			Username: "alice",
			Password: "correct password",
		})
		_, refreshErr := service.Refresh(ctx, &emptypb.Empty{})
		_, logoutErr := service.Logout(ctx, &emptypb.Empty{})
		for _, err := range []error{loginErr, refreshErr, logoutErr} {
			if code := kratoserrors.FromError(err).Code; code != http.StatusForbidden {
				t.Fatalf("origin %q status = %d", origin, code)
			}
		}
	}
}

func TestDisabledUserCannotRefresh(t *testing.T) {
	service, users, refresh := testService(t)
	loginContext, loginTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	if _, err := service.Login(loginContext, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	}); err != nil {
		t.Fatal(err)
	}
	cookie := responseCookie(t, loginTransport)
	users.mu.Lock()
	users.byName["alice"].Enabled = false
	users.mu.Unlock()
	ctx, _ := requestContext(http.MethodPost, "https://kirby.example.com", cookie.String())
	_, err := service.Refresh(ctx, &emptypb.Empty{})
	if code := kratoserrors.FromError(err).Code; code != http.StatusUnauthorized {
		t.Fatalf("disabled refresh status = %d", code)
	}
	if refresh.activeCount() != 0 {
		t.Fatal("disabled user's rotated session remains active")
	}
}

func TestLoginThroughTrustedTLSProxySetsSecureCookie(t *testing.T) {
	service, _, _ := testServiceWithTrustedProxies(t, []string{"10.0.0.0/8"})
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requestTransport.request.RemoteAddr = "10.0.0.2:443"
	requestTransport.request.TLS = nil
	requestTransport.request.Host = "kirby:8080"
	requestTransport.request.Header.Set("X-Forwarded-For", "198.51.100.10")
	requestTransport.request.Header.Set("X-Forwarded-Proto", "https")
	requestTransport.request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	if _, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	}); err != nil {
		t.Fatal(err)
	}
	if !responseCookie(t, requestTransport).Secure {
		t.Fatal("public HTTPS session must set a Secure refresh cookie")
	}
}

func TestLoginIgnoresUntrustedForwardedOrigin(t *testing.T) {
	service, _, refresh := testServiceWithTrustedProxies(t, []string{})
	ctx, requestTransport := requestContext(http.MethodPost, "https://evil.example.com", "")
	requestTransport.request.Header.Set("X-Forwarded-Proto", "https")
	requestTransport.request.Header.Set("X-Forwarded-Host", "evil.example.com")
	_, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	})
	if code := kratoserrors.FromError(err).Code; code != http.StatusForbidden {
		t.Fatalf("untrusted forwarded origin status = %d", code)
	}
	if len(refresh.records) != 0 {
		t.Fatal("untrusted forwarded origin created a session")
	}
}

func TestLoginThroughDefaultProxyPolicyNeedsNoCIDRConfiguration(t *testing.T) {
	service, _, _ := testService(t)
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requestTransport.request.RemoteAddr = "172.20.0.2:443"
	requestTransport.request.TLS = nil
	requestTransport.request.Host = "kirby:8080"
	requestTransport.request.Header.Set("X-Forwarded-Proto", "https")
	requestTransport.request.Header.Set("X-Forwarded-Host", "kirby.example.com")
	if _, err := service.Login(ctx, &adminv1.LoginRequest{
		Username: "alice",
		Password: "correct password",
	}); err != nil {
		t.Fatalf("default proxy configuration login: %v", err)
	}
	if !responseCookie(t, requestTransport).Secure {
		t.Fatal("default HTTPS proxy must set a Secure refresh cookie")
	}
}

type profileAuthLogic struct {
	authLogic
	profile *logic.AuthProfile
}

func (l profileAuthLogic) Me(context.Context, int64) (*logic.AuthProfile, error) {
	return l.profile, nil
}

func TestAuthenticatedProfileKeepsEnvironmentProjectOwnership(t *testing.T) {
	profile := &logic.AuthProfile{
		User: &model.User{
			Meta:    model.Meta{ID: 9},
			Enabled: true,
		},
		Environments: []model.Environment{{
			Meta:      model.Meta{ID: 2},
			ProjectID: 40,
			Key:       "prod",
			Enabled:   true,
		}},
	}
	service := &AuthService{authLogic: profileAuthLogic{profile: profile}}
	ctx, tr := requestContext(http.MethodGet, "", "")
	tr.request.Header.Set("Authorization", "Bearer test-token")
	handler := middleware.AdminAuth(environmentTestVerifier{}, environmentTestUsers{})(func(ctx context.Context, _ any) (any, error) {
		return service.Me(ctx, &emptypb.Empty{})
	})
	response, err := handler(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	reply := response.(*adminv1.MeReply)
	require.Len(t, reply.Environments, 1)
	require.Equal(t, int64(40), reply.Environments[0].ProjectId)
	want, err := converter.EnvironmentToProto(&profile.Environments[0])
	require.NoError(t, err)
	require.Equal(t, want, reply.Environments[0])
}
