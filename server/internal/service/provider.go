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
	NewEnvironmentPermissionService,
	NewExportService,
	NewImportService,
	NewProjectService,
	NewPublishService,
	NewRuntimeService,
	NewSnapshotService,
	NewStructureService,
	NewUserService,
	NewSystemSettingsService,
	NewProjectMemberService,
	NewProjectInvitationService,
	NewTransferService,
)
