package repository

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type ProjectAPIKeyRepository struct {
	engine *xorm.Engine
}

func NewProjectAPIKeyRepository(engine *xorm.Engine) *ProjectAPIKeyRepository {
	return &ProjectAPIKeyRepository{engine: engine}
}

func (r *ProjectAPIKeyRepository) CreateTx(ctx context.Context, tx *xorm.Session, environmentID, projectID int64, key *model.ProjectAPIKey) error {
	if err := validateAPIKeyScope(environmentID, projectID); err != nil {
		return err
	}
	if err := validateAPIKeyRecord(key, projectID); err != nil {
		return err
	}
	result, err := base.ExecuteTx(ctx, tx, "project API key", `
INSERT INTO project_api_keys
    (project_id, public_id, name, secret_hash, secret_suffix, created_by,
     last_used_at, revoked_at, created_at, updated_at)
SELECT p.id, ?, ?, ?, ?, ?, NULL, NULL, ?, ?
FROM projects AS p
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.id = ? AND p.deleted_at IS NULL`,
		key.PublicID, key.Name, key.SecretHash, key.SecretSuffix, key.CreatedBy,
		key.CreatedAt, key.UpdatedAt, environmentID, projectID)
	if err != nil {
		return classifyKeyWriteError("project API key", err)
	}
	key.ID, err = result.LastInsertId()
	if err != nil {
		return base.Wrap("read inserted project API key id", err)
	}
	return nil
}

func (r *ProjectAPIKeyRepository) List(ctx context.Context, environmentID, projectID int64) ([]model.ProjectAPIKey, error) {
	if err := validateAPIKeyScope(environmentID, projectID); err != nil {
		return nil, err
	}
	items := make([]model.ProjectAPIKey, 0)
	err := base.FindAll(ctx, r.engine, "project API keys", `
SELECT k.*
FROM project_api_keys AS k
INNER JOIN projects AS p ON p.id = k.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.id = ?
ORDER BY k.id DESC`, []any{environmentID, projectID}, &items)
	return items, err
}

func (r *ProjectAPIKeyRepository) FindByID(ctx context.Context, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error) {
	if err := validateAPIKeyIdentity(environmentID, projectID, keyID); err != nil {
		return nil, err
	}
	var key model.ProjectAPIKey
	err := base.FindOne(ctx, r.engine, "project API key", scopedProjectKeyLookupSQL, []any{environmentID, projectID, keyID}, &key)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

const scopedProjectKeyLookupSQL = `
SELECT k.*
FROM project_api_keys AS k
INNER JOIN projects AS p ON p.id = k.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
WHERE e.id = ? AND p.id = ? AND k.id = ?
LIMIT 1`

func (r *ProjectAPIKeyRepository) LockByID(ctx context.Context, tx *xorm.Session, environmentID, projectID, keyID int64) (*model.ProjectAPIKey, error) {
	if err := validateAPIKeyIdentity(environmentID, projectID, keyID); err != nil {
		return nil, err
	}
	var key model.ProjectAPIKey
	err := base.LockOne(ctx, tx, "project API key", `
SELECT k.*
FROM project_api_keys AS k
WHERE k.project_id = ? AND k.id = ?
  AND EXISTS (
      SELECT 1
      FROM projects AS p
      INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
      WHERE e.id = ? AND p.id = k.project_id AND p.deleted_at IS NULL
  )
LIMIT 1
FOR UPDATE`, []any{projectID, keyID, environmentID}, &key)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *ProjectAPIKeyRepository) RotateTx(ctx context.Context, tx *xorm.Session, environmentID, projectID, keyID int64, currentPublicID string, replacement *model.ProjectAPIKey) error {
	if err := validateAPIKeyIdentity(environmentID, projectID, keyID); err != nil {
		return err
	}
	if strings.TrimSpace(currentPublicID) == "" {
		return base.InvalidArgument("current public id is empty")
	}
	if err := validateAPIKeyRecord(replacement, projectID); err != nil {
		return err
	}
	_, err := base.ExecuteTx(ctx, tx, "project API key rotation", `
UPDATE project_api_keys AS k
INNER JOIN projects AS p ON p.id = k.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
SET k.public_id = ?, k.secret_hash = ?, k.secret_suffix = ?,
    k.last_used_at = NULL, k.updated_at = ?
WHERE e.id = ? AND p.id = ? AND k.id = ?
  AND k.public_id = ? AND k.revoked_at IS NULL`,
		replacement.PublicID, replacement.SecretHash, replacement.SecretSuffix, replacement.UpdatedAt,
		environmentID, projectID, keyID, currentPublicID)
	return classifyKeyWriteError("project API key", err)
}

func (r *ProjectAPIKeyRepository) RevokeTx(ctx context.Context, tx *xorm.Session, environmentID, projectID, keyID int64, currentPublicID string, revokedAt time.Time) error {
	if err := validateAPIKeyIdentity(environmentID, projectID, keyID); err != nil {
		return err
	}
	if strings.TrimSpace(currentPublicID) == "" || revokedAt.IsZero() {
		return base.InvalidArgument("current public id and revoked_at are required")
	}
	_, err := base.ExecuteTx(ctx, tx, "project API key revocation", `
UPDATE project_api_keys AS k
INNER JOIN projects AS p ON p.id = k.project_id AND p.deleted_at IS NULL
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL
SET k.revoked_at = ?, k.updated_at = ?
WHERE e.id = ? AND p.id = ? AND k.id = ?
  AND k.public_id = ? AND k.revoked_at IS NULL`,
		revokedAt, revokedAt, environmentID, projectID, keyID, currentPublicID)
	return err
}

// LockRuntimeCredential obtains a shared lock for the whole runtime read. A
// rotation or revocation therefore cannot commit before a request using the
// previous credential finishes.
func (r *ProjectAPIKeyRepository) LockRuntimeCredential(ctx context.Context, tx *xorm.Session, publicID string) (*model.ProjectAPIKey, error) {
	if strings.TrimSpace(publicID) == "" || len(publicID) > 64 {
		return nil, base.InvalidArgument("public id is invalid")
	}
	var key model.ProjectAPIKey
	err := base.LockOne(ctx, tx, "runtime API key", `
SELECT k.*
FROM project_api_keys AS k
WHERE k.public_id = ?
LIMIT 1
FOR SHARE`, []any{publicID}, &key)
	if err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *ProjectAPIKeyRepository) FindRuntimeProjectTx(ctx context.Context, tx *xorm.Session, projectID int64) (*model.Project, error) {
	if err := base.ValidateID("project_id", projectID); err != nil {
		return nil, err
	}
	var project model.Project
	err := base.LockOne(ctx, tx, "runtime project", `
SELECT p.*
FROM projects AS p
INNER JOIN environments AS e ON e.project_id = p.id AND e.deleted_at IS NULL AND e.enabled = TRUE
WHERE p.id = ? AND p.deleted_at IS NULL
LIMIT 1
FOR SHARE`, []any{projectID}, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *ProjectAPIKeyRepository) FindRuntimeConfigTx(ctx context.Context, tx *xorm.Session, projectID int64, configKey string) (*model.Config, error) {
	if err := base.ValidateID("project_id", projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(configKey) == "" || len(configKey) > 64 {
		return nil, base.InvalidArgument("config key is invalid")
	}
	var config model.Config
	err := base.LockOne(ctx, tx, "runtime config", `
SELECT c.*
FROM configs AS c
WHERE c.project_id = ? AND c.`+"`key`"+` = ? AND c.deleted_at IS NULL
LIMIT 1
FOR SHARE`, []any{projectID, configKey}, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *ProjectAPIKeyRepository) FindReleasedSnapshotTx(ctx context.Context, tx *xorm.Session, projectID, configID int64) (*model.Snapshot, error) {
	if err := base.ValidateID("project_id", projectID); err != nil {
		return nil, err
	}
	if err := base.ValidateID("config_id", configID); err != nil {
		return nil, err
	}
	var snapshot model.Snapshot
	err := base.LockOne(ctx, tx, "released runtime snapshot", `
SELECT s.*
FROM snapshots AS s
WHERE s.project_id = ? AND s.config_id = ? AND s.status = ? AND s.deleted_at IS NULL
ORDER BY s.id DESC
LIMIT 1
FOR SHARE`, []any{projectID, configID, model.SnapshotStatusReleased}, &snapshot)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *ProjectAPIKeyRepository) MarkUsed(ctx context.Context, publicID string, usedAt time.Time) error {
	if strings.TrimSpace(publicID) == "" || len(publicID) > 64 || usedAt.IsZero() {
		return base.InvalidArgument("public id and used_at are required")
	}
	if err := base.ValidateEngine(r.engine == nil); err != nil {
		return err
	}
	_, err := r.engine.Context(ctx).Exec(`
UPDATE project_api_keys
SET last_used_at = IF(last_used_at IS NULL OR last_used_at < ?, ?, last_used_at),
    updated_at = IF(updated_at < ?, ?, updated_at)
WHERE public_id = ? AND revoked_at IS NULL`, usedAt, usedAt, usedAt, usedAt, publicID)
	if err != nil {
		return base.Wrap("mark project API key used", err)
	}
	return nil
}

func validateAPIKeyScope(environmentID, projectID int64) error {
	return validateEnvironmentResource(environmentID, "project_id", projectID)
}

func validateAPIKeyIdentity(environmentID, projectID, keyID int64) error {
	if err := validateAPIKeyScope(environmentID, projectID); err != nil {
		return err
	}
	return base.ValidateID("key_id", keyID)
}

func validateAPIKeyRecord(key *model.ProjectAPIKey, projectID int64) error {
	if key == nil {
		return base.InvalidArgument("project API key is nil")
	}
	if key.ProjectID != 0 && key.ProjectID != projectID {
		return base.InvalidArgument("project API key project_id does not match")
	}
	if strings.TrimSpace(key.PublicID) == "" || len(key.PublicID) > 64 {
		return base.InvalidArgument("project API key public id is invalid")
	}
	nameLength := utf8.RuneCountInString(key.Name)
	if strings.TrimSpace(key.Name) == "" || nameLength > 64 {
		return base.InvalidArgument("project API key name is invalid")
	}
	if len(key.SecretHash) != sha256.Size || len(key.SecretSuffix) != 4 {
		return base.InvalidArgument("project API key digest is invalid")
	}
	if key.CreatedBy <= 0 || key.CreatedAt.IsZero() || key.UpdatedAt.IsZero() {
		return base.InvalidArgument("project API key metadata is invalid")
	}
	key.ProjectID = projectID
	return nil
}
