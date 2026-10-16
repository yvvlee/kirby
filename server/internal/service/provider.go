package service

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAPIKeyService,
	NewAssetService,
	NewAuthService,
	NewSetupService,
	NewConfigService,
	NewEnumService,
	NewEnvironmentService,
	NewExportService,
	NewImportService,
	NewProjectService,
	NewPublishService,
	NewRoleService,
	NewRuntimeService,
	NewSnapshotService,
	NewStructureService,
	NewUserService,
	NewTransferService,
)
