package permission

import (
	"context"
	"net/http"
	"strings"
	"testing"

	kratosmiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	adminmiddleware "github.com/yvvlee/kirby/server/internal/middleware"
	"github.com/yvvlee/kirby/server/internal/model"
)

type actorVerifier struct{}

func (actorVerifier) Verify(string) (*authjwt.Claims, error) {
	return &authjwt.Claims{SessionID: "session", RegisteredClaims: jwtlib.RegisteredClaims{Subject: "7"}}, nil
}

type actorUsers struct{}

func (actorUsers) GetByID(context.Context, int64) (*model.User, error) {
	return &model.User{Meta: model.Meta{ID: 7}, Enabled: true}, nil
}

type actorTransport struct{ header actorHeader }

func (actorTransport) Kind() transport.Kind              { return transport.KindHTTP }
func (actorTransport) Endpoint() string                  { return "http://localhost" }
func (actorTransport) Operation() string                 { return "test" }
func (t actorTransport) RequestHeader() transport.Header { return t.header }
func (actorTransport) ReplyHeader() transport.Header     { return actorHeader{} }

type actorHeader http.Header

func (h actorHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h actorHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h actorHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h actorHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h actorHeader) Keys() []string {
	result := make([]string, 0, len(h))
	for key := range h {
		result = append(result, key)
	}
	return result
}

func actorFromRequestID(t *testing.T, requestID string) Actor {
	t.Helper()
	header := actorHeader{}
	header.Set("Authorization", "Bearer access-token")
	header.Set("X-Request-ID", requestID)
	ctx := transport.NewServerContext(context.Background(), actorTransport{header: header})
	var actor Actor
	handler := func(ctx context.Context, _ any) (any, error) {
		var err error
		actor, err = ActorFromContext(ctx)
		return nil, err
	}
	_, err := adminmiddleware.AdminAuth(actorVerifier{}, actorUsers{})(kratosmiddleware.Handler(handler))(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func TestActorFromContextAcceptsOnlySafeRequestID(t *testing.T) {
	valid := "request.AZ_09:part-1"
	if actor := actorFromRequestID(t, valid); actor.RequestID != valid {
		t.Fatalf("valid request id replaced: %q", actor.RequestID)
	}
	for _, invalid := range []string{"line\nbreak", "space id", "非ASCII", strings.Repeat("a", 129)} {
		actor := actorFromRequestID(t, invalid)
		if actor.RequestID == invalid {
			t.Fatalf("unsafe request id retained: %q", invalid)
		}
		if _, err := uuid.Parse(actor.RequestID); err != nil {
			t.Fatalf("replacement request id %q is not a UUID: %v", actor.RequestID, err)
		}
	}
}
