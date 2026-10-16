package service

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yvvlee/lorm"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/auth"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

func TestInitialAdministratorCanLoginAndRefresh(t *testing.T) {
	dsn := os.Getenv("KIRBY_SETUP_LOGIN_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set KIRBY_SETUP_LOGIN_MYSQL_DSN to a disposable empty database")
	}
	database, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	ctx := context.Background()
	require.NoError(t, model.InitializeDatabase(ctx, database, false))
	engine, err := lorm.NewEngine("mysql", dsn, lorm.WithLogger(nil))
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })
	store := &infra.Storage{
		CoreDB: engine,
		Cache:  cache.NewMemory(),
	}
	cfg := &conf.Config{
		JWT: conf.JWTConfig{
			Issuer:     "setup-login-test",
			AccessTTL:  conf.Duration{Duration: 15 * time.Minute},
			RefreshTTL: conf.Duration{Duration: 7 * 24 * time.Hour},
		},
	}
	hasher := password.NewDefault()
	setupLogic := logic.NewSetupLogic(repository.NewSetupRepository(store), hasher, store.CoreDB)
	require.NoError(t, setupLogic.CreateInitialAdmin(ctx, "setup-owner", "Setup Owner", "integration-password"))
	secrets, found, err := engine.Query[*model.ApplicationSecrets]().ID(1).Get(ctx)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, secrets)
	tokens, err := auth.ProvideJWTManager(cfg, secrets)
	require.NoError(t, err)
	users := repository.NewUserRepository(store)
	refresh := repository.NewRefreshTokenRepository(store)
	authLogic, err := logic.NewAuthLogic(cfg, users, refresh, store.Cache, store.CoreDB, hasher, tokens)
	require.NoError(t, err)
	service, err := NewAuthService(cfg, authLogic)
	require.NoError(t, err)
	loginCtx, loginTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	reply, err := service.Login(loginCtx, &adminv1.LoginRequest{
		Username: "setup-owner",
		Password: "integration-password",
	})
	require.NoError(t, err)
	require.NotEmpty(t, reply.AccessToken)
	require.True(t, reply.User.IsSystemAdmin)
	cookie := responseCookie(t, loginTransport)
	refreshCtx, _ := requestContext(http.MethodPost, "https://kirby.example.com", cookie.String())
	refreshed, err := service.Refresh(refreshCtx, &emptypb.Empty{})
	require.NoError(t, err)
	require.NotEmpty(t, refreshed.AccessToken)
}
