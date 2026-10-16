package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type APIKeyMutation struct {
	EnvironmentID   int64
	ProjectID       int64
	KeyID           int64
	CurrentPublicID string
}

type ProjectAPIKeyRepository interface {
	Create(ctx context.Context, environmentID, projectID int64, key *model.ProjectAPIKey) error
	List(ctx context.Context, environmentID, projectID int64) ([]model.ProjectAPIKey, error)
	FindByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error)
	LockByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error)
	Rotate(ctx context.Context, mutation APIKeyMutation, replacement *model.ProjectAPIKey) error
	Revoke(ctx context.Context, mutation APIKeyMutation, revokedAt time.Time) error
	LockRuntimeCredential(ctx context.Context, publicID string) (*model.ProjectAPIKey, error)
	FindRuntimeProject(ctx context.Context, projectID int64) (*model.Project, error)
	FindRuntimeConfig(ctx context.Context, projectID int64, configKey string) (*model.Config, error)
	FindReleasedSnapshot(ctx context.Context, projectID, configID int64) (*model.Snapshot, error)
	MarkUsed(ctx context.Context, publicID string, usedAt time.Time) error
}

type ProjectAPIKeyRepositoryImpl struct {
	*lorm.Repository[*model.ProjectAPIKey]
}

// NewProjectAPIKeyRepository binds the ProjectAPIKeyRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured ProjectAPIKeyRepository.
// Throws: none.
func NewProjectAPIKeyRepository(storage *infra.Storage) *ProjectAPIKeyRepositoryImpl {
	return &ProjectAPIKeyRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.ProjectAPIKey](),
	}
}

// Create locks project ownership before storing a new API credential.
//
// Parameters: ctx carries the transaction; environmentID and projectID define ownership; key supplies the generated public ID and digest.
// Returns: nil after insertion, ErrNotFound for an invalid scope, ErrKeyConflict, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Create(ctx context.Context, environmentID, projectID int64, key *model.ProjectAPIKey) error {
	if err := lockScopedProject(ctx, r.Engine, environmentID, projectID); err != nil {
		return err
	}
	key.ProjectID = projectID
	key.PrepareInsert()
	_, err := r.Insert(ctx, key)
	return classifyKeyWriteError("project API key", err)
}

// List lists credentials only through a live project and environment association.
//
// Parameters: ctx controls the read; environmentID and projectID define the scope.
// Returns: credentials ordered by descending ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) List(ctx context.Context, environmentID, projectID int64) ([]model.ProjectAPIKey, error) {
	items, err := apiKeyScope(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"p.id": projectID,
	}).Desc("k.id").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(items), nil
}

// FindByID keeps credential existence private outside its environment and project.
//
// Parameters: ctx controls the read; environmentID, projectID, and keyID identify the scoped credential.
// Returns: the credential, ErrNotFound for missing or foreign resources, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error) {
	key, found, err := apiKeyScope(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"p.id": projectID,
		"k.id": keyID,
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project API key")
	}
	return key, nil
}

// LockByID locks a scoped credential so competing rotation and revocation serialize.
//
// Parameters: ctx carries the transaction; environmentID, projectID, and keyID identify the scoped credential.
// Returns: the locked credential, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) LockByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error) {
	key, found, err := apiKeyScope(r.Engine).Where(builder.Eq{
		"e.id": environmentID,
		"p.id": projectID,
		"k.id": keyID,
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project API key")
	}
	return key, nil
}

// Rotate replaces credentials only when the public ID and live ownership still match.
//
// Parameters: ctx carries the transaction; mutation identifies the scope and expected public ID; replacement supplies the new credential fields.
// Returns: nil after one unrevoked credential changes, ErrNoRowsAffected, ErrKeyConflict, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Rotate(ctx context.Context, mutation APIKeyMutation, replacement *model.ProjectAPIKey) error {
	fields := new(model.ProjectAPIKey).LormCols()
	affected, err := r.Engine.Update[*model.ProjectAPIKey]().ID(mutation.KeyID).
		Where(apiKeyWriteScope(mutation.EnvironmentID, mutation.ProjectID, mutation.CurrentPublicID)).SetMap(map[string]any{
		fields.PublicID():     replacement.PublicID,
		fields.SecretHash():   replacement.SecretHash,
		fields.SecretSuffix(): replacement.SecretSuffix,
		fields.LastUsedAt():   nil,
		fields.UpdatedAt():    replacement.UpdatedAt.UTC(),
	}).Exec(ctx)
	if err != nil {
		return classifyKeyWriteError("project API key", err)
	}
	if affected == 0 {
		return Unchanged("project API key rotation")
	}
	return nil
}

// Revoke revokes a credential only when its public ID and live ownership still match.
//
// Parameters: ctx carries the transaction; mutation identifies the scope and expected public ID; revokedAt is the revocation time.
// Returns: nil after one credential changes, ErrNoRowsAffected, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Revoke(ctx context.Context, mutation APIKeyMutation, revokedAt time.Time) error {
	revokedAt = revokedAt.UTC()
	fields := new(model.ProjectAPIKey).LormCols()
	affected, err := r.Engine.Update[*model.ProjectAPIKey]().ID(mutation.KeyID).
		Where(apiKeyWriteScope(mutation.EnvironmentID, mutation.ProjectID, mutation.CurrentPublicID)).SetMap(map[string]any{
		fields.RevokedAt(): revokedAt,
		fields.UpdatedAt(): revokedAt,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("project API key revocation")
	}
	return nil
}

// Keep the environment constraint in the write, even when the caller has already locked the key.
func apiKeyWriteScope(environmentID, projectID int64, publicID string) builder.Sqlizer {
	fields := new(model.ProjectAPIKey).LormCols()
	return builder.And{
		builder.And{
			builder.Eq{
				fields.ProjectID(): projectID,
				fields.PublicID():  publicID,
			},
			builder.IsNull(fields.RevokedAt()),
		},
		builder.Expr("EXISTS (SELECT 1 FROM projects p JOIN environments e ON e.project_id = p.id WHERE p.id = ? AND e.id = ? AND p.deleted_at IS NULL AND e.deleted_at IS NULL)", projectID, environmentID),
	}
}

// LockRuntimeCredential prevents rotation and revocation from racing an authenticated runtime read.
//
// Parameters: ctx carries the runtime transaction; publicID selects the credential.
// Returns: the locked credential, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) LockRuntimeCredential(ctx context.Context, publicID string) (*model.ProjectAPIKey, error) {
	key, found, err := r.Engine.Query[*model.ProjectAPIKey]().
		Where(builder.Eq{new(model.ProjectAPIKey).LormCols().PublicID(): publicID}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("runtime API key")
	}
	return key, nil
}

// FindRuntimeProject requires a live project with an enabled environment before runtime access.
//
// Parameters: ctx carries the runtime transaction; projectID selects the authenticated project.
// Returns: the locked project, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindRuntimeProject(ctx context.Context, projectID int64) (*model.Project, error) {
	project, found, err := r.Engine.Query[*model.Project]().From("projects p").Select("p.*").
		InnerJoin("environments e ON e.project_id = p.id AND e.enabled = TRUE AND e.deleted_at IS NULL").
		Where(builder.And{
			builder.Eq{
				"p.id": projectID,
			},
			builder.IsNull("p.deleted_at"),
		}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("runtime project")
	}
	return project, nil
}

// FindRuntimeConfig locks a live configuration within the authenticated project.
//
// Parameters: ctx carries the runtime transaction; projectID defines ownership; configKey selects the configuration.
// Returns: the locked configuration, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindRuntimeConfig(ctx context.Context, projectID int64, configKey string) (*model.Config, error) {
	fields := new(model.Config).LormCols()
	config, found, err := r.Engine.Query[*model.Config]().Where(builder.And{
		builder.Eq{
			fields.ProjectID(): projectID,
			fields.Key():       configKey,
		},
		builder.IsNull(fields.DeletedAt()),
	}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("runtime config")
	}
	return config, nil
}

// FindReleasedSnapshot locks the latest live release for the authenticated configuration.
//
// Parameters: ctx carries the runtime transaction; projectID and configID select the release scope.
// Returns: the latest released snapshot, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindReleasedSnapshot(ctx context.Context, projectID, configID int64) (*model.Snapshot, error) {
	fields := new(model.Snapshot).LormCols()
	snapshot, found, err := r.Engine.Query[*model.Snapshot]().Where(builder.And{
		builder.Eq{
			fields.ProjectID(): projectID,
			fields.ConfigID():  configID,
			fields.Status():    model.SnapshotStatusReleased,
		},
		builder.IsNull(fields.DeletedAt()),
	}).Desc(fields.ID()).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("released runtime snapshot")
	}
	return snapshot, nil
}

// MarkUsed keeps usage timestamps monotonic without reviving revoked credentials.
//
// Parameters: ctx controls the write; publicID selects the credential; usedAt is the observed access time.
// Returns: nil when updated or already newer/revoked, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) MarkUsed(ctx context.Context, publicID string, usedAt time.Time) error {
	usedAt = usedAt.UTC()
	fields := new(model.ProjectAPIKey).LormCols()
	_, err := r.Engine.Update[*model.ProjectAPIKey]().Where(builder.And{
		builder.Eq{
			fields.PublicID(): publicID,
		},
		builder.IsNull(fields.RevokedAt()),
	}).Where(builder.Or{
		builder.IsNull(fields.LastUsedAt()),
		builder.Lt(fields.LastUsedAt(), usedAt),
	}).SetMap(map[string]any{
		fields.LastUsedAt(): usedAt,
		fields.UpdatedAt():  usedAt,
	}).Exec(ctx)
	return WrapDatabaseError(err)
}

var _ ProjectAPIKeyRepository = (*ProjectAPIKeyRepositoryImpl)(nil)
