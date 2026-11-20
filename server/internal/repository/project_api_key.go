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
	EnvironmentID     int64
	ProjectID         int64
	KeyID             int64
	CurrentSecretHash []byte
}

type ProjectAPIKeyRepository interface {
	Create(ctx context.Context, environmentID, projectID int64, key *model.ProjectAPIKey) (rowsAffected int64, err error)
	List(ctx context.Context, environmentID, projectID int64) ([]model.ProjectAPIKey, error)
	FindByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error)
	LockByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error)
	Rotate(ctx context.Context, mutation APIKeyMutation, replacement *model.ProjectAPIKey) (rowsAffected int64, err error)
	Revoke(ctx context.Context, mutation APIKeyMutation, revokedAt time.Time) (rowsAffected int64, err error)
	LockRuntimeCredential(ctx context.Context, secretHash []byte) (*model.ProjectAPIKey, error)
	FindRuntimeProject(ctx context.Context, environmentID, projectID int64) (*model.Project, error)
	FindRuntimeConfig(ctx context.Context, environmentID, projectID int64, configKey string) (*model.Config, error)
	FindReleasedSnapshot(ctx context.Context, projectID, configID int64) (*model.Snapshot, error)
	MarkUsed(ctx context.Context, secretHash []byte, usedAt time.Time) (rowsAffected int64, err error)
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
// Parameters: ctx carries the transaction; environmentID and projectID define ownership; key supplies the generated credential digest and digest.
// Returns: rowsAffected from the credential insert, ErrNotFound for an invalid scope, ErrKeyConflict, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Create(ctx context.Context, environmentID, projectID int64, key *model.ProjectAPIKey) (rowsAffected int64, err error) {
	if err = lockScopedProject(ctx, r.Engine, environmentID, projectID); err != nil {
		return
	}
	key.EnvironmentID = environmentID
	key.ProjectID = projectID
	rowsAffected, err = r.Insert(ctx, key)
	err = classifyKeyWriteError("project API key", err)
	return
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

// Rotate replaces credentials only when the credential digest and live ownership still match.
//
// Parameters: ctx carries the transaction; mutation identifies the scope and expected credential digest; replacement supplies the new credential fields.
// Returns: rowsAffected after one unrevoked credential changes, ErrNoRowsAffected, ErrKeyConflict, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Rotate(ctx context.Context, mutation APIKeyMutation, replacement *model.ProjectAPIKey) (rowsAffected int64, err error) {
	fields := new(model.ProjectAPIKey).LormCols()
	rowsAffected, err = r.Engine.Update[*model.ProjectAPIKey]().ID(mutation.KeyID).
		Where(apiKeyWriteScope(mutation.EnvironmentID, mutation.ProjectID, mutation.CurrentSecretHash)).SetMap(map[string]any{
		fields.SecretHash():   replacement.SecretHash,
		fields.SecretSuffix(): replacement.SecretSuffix,
		fields.LastUsedAt():   nil,
		fields.UpdatedAt():    replacement.UpdatedAt.UTC(),
	}).Exec(ctx)
	if err != nil {
		err = classifyKeyWriteError("project API key", err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("project API key rotation")
		return
	}
	return
}

// Revoke revokes a credential only when its credential digest and live ownership still match.
//
// Parameters: ctx carries the transaction; mutation identifies the scope and expected credential digest; revokedAt is the revocation time.
// Returns: rowsAffected after one credential changes, ErrNoRowsAffected, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) Revoke(ctx context.Context, mutation APIKeyMutation, revokedAt time.Time) (rowsAffected int64, err error) {
	revokedAt = revokedAt.UTC()
	fields := new(model.ProjectAPIKey).LormCols()
	rowsAffected, err = r.Engine.Update[*model.ProjectAPIKey]().ID(mutation.KeyID).
		Where(apiKeyWriteScope(mutation.EnvironmentID, mutation.ProjectID, mutation.CurrentSecretHash)).SetMap(map[string]any{
		fields.RevokedAt(): revokedAt,
		fields.UpdatedAt(): revokedAt,
	}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
		return
	}
	if rowsAffected == 0 {
		err = Unchanged("project API key revocation")
		return
	}
	return
}

// Keep the environment constraint in the write, even when the caller has already locked the key.
func apiKeyWriteScope(environmentID, projectID int64, secretHash []byte) builder.Sqlizer {
	fields := new(model.ProjectAPIKey).LormCols()
	return builder.And{
		builder.And{
			builder.Eq{
				fields.ProjectID():     projectID,
				fields.SecretHash():    secretHash,
				fields.EnvironmentID(): environmentID,
			},
			builder.IsNull(fields.RevokedAt()),
		},
		builder.Expr("EXISTS (SELECT 1 FROM projects p JOIN environments e ON e.project_id = p.id WHERE p.id = ? AND e.id = ?)", projectID, environmentID),
	}
}

// LockRuntimeCredential prevents rotation and revocation from racing an authenticated runtime read.
//
// Parameters: ctx carries the runtime transaction; secretHash selects the credential.
// Returns: the locked credential, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) LockRuntimeCredential(ctx context.Context, secretHash []byte) (*model.ProjectAPIKey, error) {
	key, found, err := r.Engine.Query[*model.ProjectAPIKey]().
		Where(builder.Eq{new(model.ProjectAPIKey).LormCols().SecretHash(): secretHash}).Suffix("FOR UPDATE").Get(ctx)
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
// Parameters: ctx carries the runtime transaction; environmentID and projectID select the credential’s environment and project.
// Returns: the locked project, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindRuntimeProject(ctx context.Context, environmentID, projectID int64) (*model.Project, error) {
	project, found, err := r.Engine.Query[*model.Project]().From("projects p").Select("p.*").
		InnerJoin("environments e ON e.project_id = p.id AND e.enabled = TRUE").
		Where(builder.Eq{
			"p.id": projectID,
			"e.id": environmentID,
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
// Parameters: ctx carries the runtime transaction; environmentID and projectID define ownership; configKey selects the configuration.
// Returns: the locked configuration, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) FindRuntimeConfig(ctx context.Context, environmentID, projectID int64, configKey string) (*model.Config, error) {
	fields := new(model.Config).LormCols()
	config, found, err := r.Engine.Query[*model.Config]().Where(builder.Eq{
		fields.ProjectID():     projectID,
		fields.Key():           configKey,
		fields.EnvironmentID(): environmentID,
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
	snapshot, found, err := r.Engine.Query[*model.Snapshot]().Where(builder.Eq{
		fields.ProjectID(): projectID,
		fields.ConfigID():  configID,
		fields.Status():    model.SnapshotStatusReleased,
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
// Parameters: ctx controls the write; secretHash selects the credential; usedAt is the observed access time.
// Returns: rowsAffected when the timestamp advanced, 0 when already newer or revoked, or a database error.
// Throws: none; failures are returned as errors.
func (r *ProjectAPIKeyRepositoryImpl) MarkUsed(ctx context.Context, secretHash []byte, usedAt time.Time) (rowsAffected int64, err error) {
	usedAt = usedAt.UTC()
	fields := new(model.ProjectAPIKey).LormCols()
	rowsAffected, err = r.Engine.Update[*model.ProjectAPIKey]().Where(builder.And{
		builder.Eq{
			fields.SecretHash(): secretHash,
		},
		builder.IsNull(fields.RevokedAt()),
	}).Where(builder.Or{
		builder.IsNull(fields.LastUsedAt()),
		builder.Lt(fields.LastUsedAt(), usedAt),
	}).SetMap(map[string]any{
		fields.LastUsedAt(): usedAt,
		fields.UpdatedAt():  usedAt,
	}).Exec(ctx)
	if err != nil {
		err = WrapDatabaseError(err)
	}
	return
}

var _ ProjectAPIKeyRepository = (*ProjectAPIKeyRepositoryImpl)(nil)
