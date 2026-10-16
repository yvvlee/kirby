package repository

import "github.com/google/wire"

// ProviderSet is repository providers.
var ProviderSet = wire.NewSet(
	NewAuditLogRepository, wire.Bind(new(AuditLogRepository), new(*AuditLogRepositoryImpl)),
	NewConfigRepository, wire.Bind(new(ConfigRepository), new(*ConfigRepositoryImpl)),
	NewConfigEnumRepository, wire.Bind(new(ConfigEnumRepository), new(*ConfigEnumRepositoryImpl)),
	NewEnvironmentRepository, wire.Bind(new(EnvironmentRepository), new(*EnvironmentRepositoryImpl)),
	NewImportRecordRepository, wire.Bind(new(ImportRecordRepository), new(*ImportRecordRepositoryImpl)),
	NewPermissionRepository, wire.Bind(new(PermissionRepository), new(*PermissionRepositoryImpl)),
	NewProjectRepository, wire.Bind(new(ProjectRepository), new(*ProjectRepositoryImpl)),
	NewProjectAPIKeyRepository, wire.Bind(new(ProjectAPIKeyRepository), new(*ProjectAPIKeyRepositoryImpl)),
	NewRefreshTokenRepository, wire.Bind(new(RefreshTokenRepository), new(*RefreshTokenRepositoryImpl)),
	NewRoleRepository, wire.Bind(new(RoleRepository), new(*RoleRepositoryImpl)),
	NewSetupRepository, wire.Bind(new(SetupRepository), new(*SetupRepositoryImpl)),
	NewSnapshotRepository, wire.Bind(new(SnapshotRepository), new(*SnapshotRepositoryImpl)),
	NewSnapshotPublicationRepository, wire.Bind(new(SnapshotPublicationRepository), new(*SnapshotPublicationRepositoryImpl)),
	NewStructureRepository, wire.Bind(new(StructureRepository), new(*StructureRepositoryImpl)),
	NewUserRepository, wire.Bind(new(UserRepository), new(*UserRepositoryImpl)),
	NewUserEnvironmentRoleRepository, wire.Bind(new(UserEnvironmentRoleRepository), new(*UserEnvironmentRoleRepositoryImpl)),
)
