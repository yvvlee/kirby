package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type setupTestLogic struct {
	initialized bool
	statusErr   error
	createErr   error
	calls       int
	username    string
	displayName string
	password    string
}

func (l *setupTestLogic) GetSetupStatus(context.Context) (bool, error) {
	return l.initialized, l.statusErr
}

func (l *setupTestLogic) CreateInitialAdmin(_ context.Context, username, displayName, password string) error {
	l.calls++
	l.username = username
	l.displayName = displayName
	l.password = password
	return l.createErr
}

func setupTestService(t *testing.T, logicLayer *setupTestLogic) *SetupService {
	t.Helper()
	service, err := NewSetupService(&conf.Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	service.logic = logicLayer
	return service
}

func initialAdminRequest() *adminv1.CreateInitialAdminRequest {
	return &adminv1.CreateInitialAdminRequest{
		Username:    "owner",
		DisplayName: "Owner",
		Password:    "initial-password",
	}
}

func TestSetupStatusNeedsNoAuthenticationAndDoesNotHideDatabaseFailure(t *testing.T) {
	logicLayer := &setupTestLogic{}
	service := setupTestService(t, logicLayer)
	reply, err := service.GetSetupStatus(context.Background(), &emptypb.Empty{})
	if err != nil || reply.Initialized {
		t.Fatalf("fresh status = %+v, error = %v", reply, err)
	}
	logicLayer.initialized = true
	reply, err = service.GetSetupStatus(context.Background(), &emptypb.Empty{})
	if err != nil || !reply.Initialized {
		t.Fatalf("initialized status = %+v, error = %v", reply, err)
	}
	logicLayer.statusErr = errors.New("database unavailable")
	if reply, err := service.GetSetupStatus(context.Background(), &emptypb.Empty{}); reply != nil || kratoserrors.FromError(err).Code != http.StatusInternalServerError {
		t.Fatalf("failed database read: reply=%+v err=%v", reply, err)
	}
}

func TestSetupCreatesInitialAdminWithoutStartingSession(t *testing.T) {
	logicLayer := &setupTestLogic{}
	service := setupTestService(t, logicLayer)
	ctx, requestTransport := requestContext(http.MethodPost, "https://kirby.example.com", "")
	if _, err := service.CreateInitialAdmin(ctx, initialAdminRequest()); err != nil {
		t.Fatal(err)
	}
	if logicLayer.calls != 1 || logicLayer.username != "owner" || logicLayer.displayName != "Owner" || logicLayer.password != "initial-password" {
		t.Fatalf("setup inputs were not passed through: %+v", logicLayer)
	}
	if len(requestTransport.reply.Values("Set-Cookie")) != 0 {
		t.Fatal("initial administrator setup started a session")
	}
}

func TestSetupRequiresSameOriginBeforeCreatingAdmin(t *testing.T) {
	logicLayer := &setupTestLogic{}
	service := setupTestService(t, logicLayer)
	for _, origin := range []string{"", "null", "https://evil.example.com", "http://kirby.example.com", "https://kirby.example.com:8443"} {
		ctx, _ := requestContext(http.MethodPost, origin, authsession.CookieName+"=existing-session")
		if _, err := service.CreateInitialAdmin(ctx, initialAdminRequest()); kratoserrors.FromError(err).Code != http.StatusForbidden {
			t.Fatalf("origin %q: %v", origin, err)
		}
	}
	if logicLayer.calls != 0 {
		t.Fatal("invalid browser origin reached setup logic")
	}
}

func TestSetupRejectsInvalidCredentialsAtBoundary(t *testing.T) {
	logicLayer := &setupTestLogic{}
	service := setupTestService(t, logicLayer)
	ctx, _ := requestContext(http.MethodPost, "https://kirby.example.com", "")
	requests := []*adminv1.CreateInitialAdminRequest{
		nil,
		{
			Username:    " ",
			DisplayName: "Owner",
			Password:    "initial-password",
		},
		{
			Username:    "owner\n",
			DisplayName: "Owner",
			Password:    "initial-password",
		},
		{
			Username:    strings.Repeat("a", 129),
			DisplayName: "Owner",
			Password:    "initial-password",
		},
		{
			Username:    "owner",
			DisplayName: " ",
			Password:    "initial-password",
		},
		{
			Username:    "owner",
			DisplayName: "Own\x00er",
			Password:    "initial-password",
		},
		{
			Username:    "owner",
			DisplayName: string([]byte{0xff}),
			Password:    "initial-password",
		},
		{
			Username:    "owner",
			DisplayName: "Owner",
			Password:    "too-short",
		},
		{
			Username:    "owner",
			DisplayName: "Owner",
			Password:    strings.Repeat("a", 1025),
		},
		{
			Username:    "owner",
			DisplayName: "Owner",
			Password:    strings.Repeat(" ", 12),
		},
		{
			Username:    "owner",
			DisplayName: "Owner",
			Password:    "initial-\x00password",
		},
	}
	for index, request := range requests {
		if _, err := service.CreateInitialAdmin(ctx, request); kratoserrors.FromError(err).Code != http.StatusBadRequest {
			t.Fatalf("invalid credential case %d: %v", index, err)
		}
	}
	if logicLayer.calls != 0 {
		t.Fatal("invalid credentials reached setup logic")
	}
}

func TestSetupReturnsConflictAfterPermanentInitialization(t *testing.T) {
	logicLayer := &setupTestLogic{createErr: repository.ErrAlreadyInitialized}
	service := setupTestService(t, logicLayer)
	ctx, _ := requestContext(http.MethodPost, "https://kirby.example.com", "")
	if _, err := service.CreateInitialAdmin(ctx, initialAdminRequest()); kratoserrors.FromError(err).Code != http.StatusConflict {
		t.Fatalf("closed setup response = %v", err)
	}
	logicLayer.createErr = errors.New("database failure")
	if _, err := service.CreateInitialAdmin(ctx, initialAdminRequest()); kratoserrors.FromError(err).Code != http.StatusInternalServerError {
		t.Fatalf("setup failure response = %v", err)
	}
}

func TestSetupPasswordLimitsUseUTF8Bytes(t *testing.T) {
	for _, value := range []string{strings.Repeat("中", 4), strings.Repeat("中", 341) + "a"} {
		request := initialAdminRequest()
		request.Password = value
		if err := validateInitialAdmin(request); err != nil {
			t.Fatalf("valid multibyte password of %d bytes: %v", len(value), err)
		}
	}
	for _, value := range []string{strings.Repeat("中", 3), strings.Repeat("中", 342)} {
		request := initialAdminRequest()
		request.Password = value
		if err := validateInitialAdmin(request); kratoserrors.FromError(err).Code != http.StatusBadRequest {
			t.Fatalf("invalid multibyte password of %d bytes: %v", len(value), err)
		}
	}
}
