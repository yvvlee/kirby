package service

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type setupLogic interface {
	GetSetupStatus(ctx context.Context) (bool, error)
	CreateInitialAdmin(ctx context.Context, username string, displayName string, plainPassword string) error
}

// SetupService exposes only the operations required before the first login.
type SetupService struct {
	logic   setupLogic
	origins *authsession.OriginValidator
}

var _ adminv1.SetupServiceHTTPServer = (*SetupService)(nil)

// NewSetupService wires setup with the shared browser origin policy.
// Parameters: cfg supplies the proxy policy; logicLayer owns persistent setup state.
// Returns: the first-run setup service.
// Throws: an error when configured proxy networks are invalid.
func NewSetupService(cfg *conf.Config, logicLayer *logic.SetupLogic) (*SetupService, error) {
	origins, err := authsession.NewOriginValidator(cfg.Security.TrustedProxies)
	if err != nil {
		return nil, err
	}
	return &SetupService{
		logic:   logicLayer,
		origins: origins,
	}, nil
}

// GetSetupStatus exposes persistent setup state without requiring a login.
// Parameters: ctx controls the database read; the request body is empty.
// Returns: whether first administrator setup is complete.
// Throws: HTTP 500 when setup state cannot be read.
func (s *SetupService) GetSetupStatus(ctx context.Context, _ *emptypb.Empty) (*adminv1.SetupStatusReply, error) {
	initialized, err := s.logic.GetSetupStatus(ctx)
	if err != nil {
		return nil, errorsv1.ErrorInternal("setup status is unavailable")
	}
	return &adminv1.SetupStatusReply{Initialized: initialized}, nil
}

// CreateInitialAdmin accepts a validated same-origin browser setup request.
// Parameters: ctx contains the HTTP request; request contains the initial administrator credentials.
// Returns: an empty response without starting an authenticated session.
// Throws: HTTP 400 for invalid input, 403 for invalid origin, 409 after setup, or 500 for persistence failure.
func (s *SetupService) CreateInitialAdmin(ctx context.Context, request *adminv1.CreateInitialAdminRequest) (*emptypb.Empty, error) {
	httpRequest, ok := kratoshttp.RequestFromServerContext(ctx)
	if !ok {
		return nil, errorsv1.ErrorBadRequest("HTTP setup request required")
	}
	if err := s.origins.Validate(httpRequest); err != nil {
		return nil, errorsv1.ErrorForbidden("request origin is not allowed")
	}
	if err := validateInitialAdmin(request); err != nil {
		return nil, err
	}
	err := s.logic.CreateInitialAdmin(ctx, request.Username, request.DisplayName, request.Password)
	if errors.Is(err, repository.ErrAlreadyInitialized) {
		return nil, errorsv1.ErrorConflict("administrator setup is already complete")
	}
	if err != nil {
		return nil, errorsv1.ErrorInternal("administrator setup failed")
	}
	return &emptypb.Empty{}, nil
}

func validateInitialAdmin(request *adminv1.CreateInitialAdminRequest) error {
	if request == nil {
		return errorsv1.ErrorBadRequest("administrator credentials are required")
	}
	if err := request.Validate(); err != nil {
		return errorsv1.ErrorBadRequest("administrator credentials are invalid")
	}
	if !validSetupName(request.Username) || !validSetupName(request.DisplayName) {
		return errorsv1.ErrorBadRequest("administrator name is invalid")
	}
	if !utf8.ValidString(request.Password) || strings.TrimSpace(request.Password) == "" || strings.IndexFunc(request.Password, unicode.IsControl) != -1 {
		return errorsv1.ErrorBadRequest("administrator password is invalid")
	}
	return nil
}

func validSetupName(value string) bool {
	if !utf8.ValidString(value) || value != strings.TrimSpace(value) || value == "" {
		return false
	}
	return strings.IndexFunc(value, unicode.IsControl) == -1
}
