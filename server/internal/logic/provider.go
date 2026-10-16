package logic

import (
	"context"

	"github.com/google/wire"
	"github.com/yvvlee/lorm"

	"github.com/yvvlee/kirby/server/internal/auth"
	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/permission"
)

// ProviderSet includes the concrete providers needed by consumer-side bindings.
// Keeping each binding beside its provider lets Wire verify the full contract.
var ProviderSet = wire.NewSet(
	infra.ProviderSet,
	auth.ProviderSet,
	permission.ProviderSet,
	wire.Bind(new(TransactionRunner), new(*lorm.Engine)),
	NewContentCache,
	NewAPIKeyLogic,
	NewAuthLogic,
	NewSetupLogic,
	ProvideAssetLogic,
	NewConfigLogic,
	NewEnumLogic,
	NewEnvironmentLogic,
	NewExportLogic,
	NewImportLogic,
	NewProjectLogic,
	NewPublishLogic,
	NewRoleLogic,
	NewRuntimeLogic,
	NewSnapshotLogic,
	NewStructureLogic,
	NewUserLogic,
	wire.Bind(new(environmentPermissionChecker), new(*permission.Resolver)),
	wire.Bind(new(environmentAuthorizer), new(*permission.Resolver)),
	wire.Bind(new(projectAuthorizer), new(*permission.Resolver)),
	wire.Bind(new(roleAuthorizer), new(*permission.Resolver)),
	wire.Bind(new(userAuthorizer), new(*permission.Resolver)),
	wire.Bind(new(userPasswordHasher), new(*password.Hasher)),
	wire.Bind(new(publishedContentCleaner), new(*ContentCache)),
)

// environmentPermissionChecker checks access within the selected environment.
type environmentPermissionChecker interface {
	Require(ctx context.Context, userID int64, environmentID int64, required ...string) error
}
