// Package provider assembles Kirby's long-lived infrastructure and services.
package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	authjwt "github.com/yvvlee/kirby/server/internal/auth/jwt"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/config"
	"github.com/yvvlee/kirby/server/internal/data/datastore"
	apikeylogic "github.com/yvvlee/kirby/server/internal/logic/api_key"
	assetlogic "github.com/yvvlee/kirby/server/internal/logic/asset"
	configlogic "github.com/yvvlee/kirby/server/internal/logic/config"
	enumlogic "github.com/yvvlee/kirby/server/internal/logic/config_enum"
	environmentlogic "github.com/yvvlee/kirby/server/internal/logic/environment"
	exportlogic "github.com/yvvlee/kirby/server/internal/logic/export"
	importlogic "github.com/yvvlee/kirby/server/internal/logic/importer"
	projectlogic "github.com/yvvlee/kirby/server/internal/logic/project"
	publishlogic "github.com/yvvlee/kirby/server/internal/logic/publish"
	rolelogic "github.com/yvvlee/kirby/server/internal/logic/role"
	runtimelogic "github.com/yvvlee/kirby/server/internal/logic/runtime"
	snapshotlogic "github.com/yvvlee/kirby/server/internal/logic/snapshot"
	structurelogic "github.com/yvvlee/kirby/server/internal/logic/structure"
	userlogic "github.com/yvvlee/kirby/server/internal/logic/user"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	apikeyservice "github.com/yvvlee/kirby/server/internal/service/api_key"
	assetservice "github.com/yvvlee/kirby/server/internal/service/asset"
	authservice "github.com/yvvlee/kirby/server/internal/service/auth"
	configservice "github.com/yvvlee/kirby/server/internal/service/config"
	enumservice "github.com/yvvlee/kirby/server/internal/service/config_enum"
	environmentservice "github.com/yvvlee/kirby/server/internal/service/environment"
	exportservice "github.com/yvvlee/kirby/server/internal/service/export"
	importservice "github.com/yvvlee/kirby/server/internal/service/importer"
	projectservice "github.com/yvvlee/kirby/server/internal/service/project"
	publishservice "github.com/yvvlee/kirby/server/internal/service/publish"
	roleservice "github.com/yvvlee/kirby/server/internal/service/role"
	runtimeservice "github.com/yvvlee/kirby/server/internal/service/runtime"
	snapshotservice "github.com/yvvlee/kirby/server/internal/service/snapshot"
	structureservice "github.com/yvvlee/kirby/server/internal/service/structure"
	userservice "github.com/yvvlee/kirby/server/internal/service/user"
	"github.com/yvvlee/kirby/server/internal/storage/database"
	"github.com/yvvlee/kirby/server/internal/storage/object"
)

// Application contains every service and resource used by the transports.
type Application struct {
	Config *config.Config
	Logger *slog.Logger
	Store  *datastore.Store
	Object object.ObjectStorage
	Tokens *authjwt.Manager
	Users  *repository.UserRepository

	APIKeys      *apikeyservice.Service
	Assets       *assetservice.Service
	Auth         *authservice.Service
	Configs      *configservice.Service
	Enums        *enumservice.Service
	Environments *environmentservice.Service
	Projects     *projectservice.Service
	Publications *publishservice.Service
	Roles        *roleservice.Service
	Runtime      *runtimeservice.Service
	Snapshots    *snapshotservice.Service
	Structures   *structureservice.Service
	Transfers    *importservice.TransferService
	UsersService *userservice.Service
}

// NewApplication connects dependencies, validates the installed schema, and assembles services.
func NewApplication(ctx context.Context, cfg *config.Config, logger *slog.Logger) (_ *Application, err error) {
	if cfg == nil || logger == nil {
		return nil, fmt.Errorf("application config and logger are required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	store, err := datastore.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, store.Close())
		}
	}()
	if err = model.ValidateSchema(ctx, store.Database); err != nil {
		return nil, fmt.Errorf("validate database schema: %w", err)
	}
	objects, err := object.Open(ctx, cfg.Mode, cfg.ObjectStorage)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, objects.Close())
		}
	}()

	audits := repository.NewAuditLogRepository(store.Database)
	configs := repository.NewConfigRepository(store.Database)
	enums := repository.NewConfigEnumRepository(store.Database)
	environments := repository.NewEnvironmentRepository(store.Database, audits)
	imports := repository.NewImportRecordRepository()
	permissionsRepo := repository.NewPermissionRepository(store.Database)
	projects := repository.NewProjectRepository(store.Database)
	projectAPIKeys := repository.NewProjectAPIKeyRepository(store.Database)
	refreshTokens, err := repository.NewRefreshTokenRepository(store.Database)
	if err != nil {
		return nil, err
	}
	roles := repository.NewRoleRepository(store.Database, audits)
	snapshots := repository.NewSnapshotRepository(store.Database)
	publications := repository.NewSnapshotPublicationRepository(store.Database)
	structures := repository.NewStructureRepository(store.Database)
	users, err := repository.NewUserRepository(store.Database)
	if err != nil {
		return nil, err
	}
	members := repository.NewUserEnvironmentRoleRepository(store.Database, audits)
	resolver, err := permission.NewResolver(permissionsRepo, store.Cache)
	if err != nil {
		return nil, err
	}
	transactor, err := database.NewTransactor(store.Database)
	if err != nil {
		return nil, err
	}
	credentials, err := credential.New(cfg.Security.APIKeyPepper)
	if err != nil {
		return nil, err
	}
	tokens, err := authjwt.New(cfg.JWT)
	if err != nil {
		return nil, err
	}
	passwords, err := password.NewDefault()
	if err != nil {
		return nil, err
	}
	contentCache, err := runtimelogic.NewContentCache(store.Cache)
	if err != nil {
		return nil, err
	}

	apiKeyLogic, err := apikeylogic.New(projectAPIKeys, credentials, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	assetLogic, err := assetlogic.New(objects, assetAuthorizer{resolver}, projectScope{projects})
	if err != nil {
		return nil, err
	}
	configLogic, err := configlogic.New(configs, structures, enums, snapshots, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	enumLogic, err := enumlogic.New(enums, configs, structures, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	environmentLogic, err := environmentlogic.New(environments, users, members, resolver)
	if err != nil {
		return nil, err
	}
	exportLogic, err := exportlogic.New(snapshots, resolver)
	if err != nil {
		return nil, err
	}
	importLogic, err := importlogic.New(imports, projects, configs, structures, enums, snapshots, resolver, audits, transactor, contentCache)
	if err != nil {
		return nil, err
	}
	projectLogic, err := projectlogic.New(projects, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	publishLogic, err := publishlogic.New(configs, structures, enums, snapshots, publications, resolver, audits, transactor, contentCache)
	if err != nil {
		return nil, err
	}
	roleLogic, err := rolelogic.New(roles, permissionsRepo, resolver)
	if err != nil {
		return nil, err
	}
	runtimeLogic, err := runtimelogic.New(projectAPIKeys, credentials, transactor, contentCache)
	if err != nil {
		return nil, err
	}
	snapshotLogic, err := snapshotlogic.New(configs, structures, enums, snapshots, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	structureLogic, err := structurelogic.New(structures, configs, enums, resolver, audits, transactor)
	if err != nil {
		return nil, err
	}
	userLogic, err := userlogic.New(users, resolver, passwords)
	if err != nil {
		return nil, err
	}

	apiKeys, err := apikeyservice.New(apiKeyLogic)
	if err != nil {
		return nil, err
	}
	assets, err := assetservice.New(assetLogic)
	if err != nil {
		return nil, err
	}
	auth, err := authservice.New(cfg, users, refreshTokens, store.Cache)
	if err != nil {
		return nil, err
	}
	configService, err := configservice.New(configLogic)
	if err != nil {
		return nil, err
	}
	enumService, err := enumservice.New(enumLogic)
	if err != nil {
		return nil, err
	}
	environmentService, err := environmentservice.New(environmentLogic)
	if err != nil {
		return nil, err
	}
	exporter, err := exportservice.New(exportLogic)
	if err != nil {
		return nil, err
	}
	importer, err := importservice.New(importLogic)
	if err != nil {
		return nil, err
	}
	transfer, err := importservice.NewTransferService(exporter, importer)
	if err != nil {
		return nil, err
	}
	projectService, err := projectservice.New(projectLogic)
	if err != nil {
		return nil, err
	}
	publicationService, err := publishservice.New(publishLogic)
	if err != nil {
		return nil, err
	}
	roleService, err := roleservice.New(roleLogic)
	if err != nil {
		return nil, err
	}
	runtimeService, err := runtimeservice.New(runtimeLogic)
	if err != nil {
		return nil, err
	}
	snapshotService, err := snapshotservice.New(snapshotLogic)
	if err != nil {
		return nil, err
	}
	structureService, err := structureservice.New(structureLogic)
	if err != nil {
		return nil, err
	}
	userService, err := userservice.New(userLogic)
	if err != nil {
		return nil, err
	}

	return &Application{
		Config: cfg, Logger: logger, Store: store, Object: objects, Tokens: tokens, Users: users,
		APIKeys: apiKeys, Assets: assets, Auth: auth, Configs: configService, Enums: enumService,
		Environments: environmentService, Projects: projectService, Publications: publicationService,
		Roles: roleService, Runtime: runtimeService, Snapshots: snapshotService, Structures: structureService,
		Transfers: transfer, UsersService: userService,
	}, nil
}

// Close releases infrastructure after both listeners have stopped.
func (a *Application) Close() error {
	if a == nil {
		return nil
	}
	var objectErr, storeErr error
	if a.Object != nil {
		objectErr = a.Object.Close()
	}
	if a.Store != nil {
		storeErr = a.Store.Close()
	}
	return errors.Join(objectErr, storeErr)
}

type assetAuthorizer struct{ resolver *permission.Resolver }

func (a assetAuthorizer) HasEnvironmentPermission(ctx context.Context, environmentID int64, key string) (bool, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return false, err
	}
	err = a.resolver.Require(ctx, actor.UserID, environmentID, key)
	if errors.Is(err, permission.ErrForbidden) {
		return false, nil
	}
	return err == nil, err
}

type projectScope struct {
	projects *repository.ProjectRepositoryImpl
}

func (p projectScope) ProjectExists(ctx context.Context, environmentID, projectID int64) (bool, error) {
	_, err := p.projects.FindByID(ctx, environmentID, projectID)
	if errors.Is(err, base.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
