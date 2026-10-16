package server

import (
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/infra/object"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/service"
)

// Dependencies contains the services and shared resources used by the transports.
type Dependencies struct {
	Config *conf.Config
	Logger *infra.Logger
	Store  *infra.Storage
	Object object.ObjectStorage
	Tokens *authjwt.Manager
	Users  repository.UserRepository

	APIKeys      *service.APIKeyService
	Assets       *service.AssetService
	Auth         *service.AuthService
	Setup        *service.SetupService
	Configs      *service.ConfigService
	Enums        *service.EnumService
	Environments *service.EnvironmentService
	Projects     *service.ProjectService
	Publications *service.PublishService
	Roles        *service.RoleService
	Runtime      *service.RuntimeService
	Snapshots    *service.SnapshotService
	Structures   *service.StructureService
	Transfers    *service.TransferService
	UsersService *service.UserService
}
