package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/model"
)

type middlewareUsers struct {
	user *model.User
	err  error
}

func (u middlewareUsers) GetByID(context.Context, int64) (*model.User, error) { return u.user, u.err }

type middlewareTransport struct{ header header }

func (middlewareTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (middlewareTransport) Endpoint() string                  { return "http://localhost" }
func (middlewareTransport) Operation() string                 { return "test" }
func (t middlewareTransport) RequestHeader() transport.Header { return t.header }
func (t middlewareTransport) ReplyHeader() transport.Header   { return header{} }

type header http.Header

func (h header) Get(key string) string { return http.Header(h).Get(key) }
func (h header) Set(key, value string) { http.Header(h).Set(key, value) }
func (h header) Add(key, value string) { http.Header(h).Add(key, value) }
func (h header) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
func (h header) Values(key string) []string { return http.Header(h).Values(key) }

func TestAdminAuthAddsStablePrincipal(t *testing.T) {
	now := time.Now().UTC()
	manager, err := authjwt.New(config.JWTConfig{
		Issuer: "test", ActiveKID: "primary", AccessTTL: config.Duration{Duration: 15 * time.Minute},
		Keys: map[string]config.Secret{"primary": config.NewSecret("01234567890123456789012345678901")},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = now
	encoded, _, err := manager.Issue(11, "session-id")
	if err != nil {
		t.Fatal(err)
	}
	h := header{}
	h.Set("Authorization", "Bearer "+encoded)
	ctx := transport.NewServerContext(context.Background(), middlewareTransport{header: h})
	var got Principal
	next := func(ctx context.Context, _ any) (any, error) {
		var ok bool
		got, ok = PrincipalFromContext(ctx)
		if !ok {
			t.Fatal("principal missing")
		}
		return "ok", nil
	}
	result, err := AdminAuth(manager, middlewareUsers{user: &model.User{Meta: model.Meta{ID: 11}, Enabled: true}})(kratosmiddleware.Handler(next))(ctx, nil)
	if err != nil || result != "ok" || got.UserID != 11 || got.SessionID != "session-id" {
		t.Fatalf("result=%v principal=%+v err=%v", result, got, err)
	}
}

func TestAdminAuthUsesUniformUnauthorizedResponse(t *testing.T) {
	manager, err := authjwt.New(config.JWTConfig{
		Issuer: "test", ActiveKID: "primary", AccessTTL: config.Duration{Duration: 15 * time.Minute},
		Keys: map[string]config.Secret{"primary": config.NewSecret("01234567890123456789012345678901")},
	})
	if err != nil {
		t.Fatal(err)
	}
	cases := []header{{}, {"Authorization": []string{"Bearer invalid"}}}
	for _, h := range cases {
		ctx := transport.NewServerContext(context.Background(), middlewareTransport{header: h})
		_, err := AdminAuth(manager, middlewareUsers{user: &model.User{Enabled: true}})(func(context.Context, any) (any, error) { return nil, nil })(ctx, nil)
		kratosErr := kratoserrors.FromError(err)
		if kratosErr.Code != http.StatusUnauthorized || kratosErr.Message != "authentication failed" {
			t.Fatalf("unexpected auth error: %+v", kratosErr)
		}
	}
}
