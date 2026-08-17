package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrLastSystemAdmin     = errors.New("cannot remove the last enabled system administrator")
	ErrUserVersionConflict = errors.New("user version conflict")
	ErrUserListLimit       = errors.New("user list exceeds fixed limit")
)

const managedUserListLimit = 1000

// UserRepository reads and mutates local administrator identities.
type UserRepository struct {
	engine *xorm.Engine
	audits AuditLogRepository
}

func NewUserRepository(engine *xorm.Engine) (*UserRepository, error) {
	if engine == nil {
		return nil, fmt.Errorf("user repository database is nil")
	}
	return &UserRepository{engine: engine, audits: NewAuditLogRepository(engine)}, nil
}

// ListManagedUsers returns a deterministic bounded system-management list.
func (r *UserRepository) ListManagedUsers(ctx context.Context) ([]model.User, error) {
	users := make([]model.User, 0)
	session := r.engine.Context(ctx)
	defer session.Close()
	if err := session.SQL(`
SELECT u.*
FROM users AS u
WHERE u.deleted_at IS NULL
ORDER BY u.id ASC
LIMIT ?`, managedUserListLimit+1).Find(&users); err != nil {
		return nil, fmt.Errorf("list managed users: %w", err)
	}
	if len(users) > managedUserListLimit {
		return nil, ErrUserListLimit
	}
	return users, nil
}

func (r *UserRepository) CreateManagedUser(ctx context.Context, user *model.User, audit *model.AuditLog) error {
	if user == nil || audit == nil || r.audits == nil || user.Username == "" || user.DisplayName == "" || user.PasswordHash == "" {
		return fmt.Errorf("managed user and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		result, err := tx.Context(ctx).Exec(`
INSERT INTO users
    (username, display_name, password_hash, enabled, is_system_admin, created_by, updated_by)
VALUES (?, ?, ?, TRUE, ?, ?, ?)`,
			user.Username, user.DisplayName, user.PasswordHash, user.IsSystemAdmin, user.CreatedBy, user.UpdatedBy)
		if err != nil {
			return fmt.Errorf("create managed user: %w", err)
		}
		user.ID, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("read managed user id: %w", err)
		}
		audit.ResourceID = fmt.Sprintf("%d", user.ID)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
}

func (r *UserRepository) UpdateManagedUser(ctx context.Context, userID int64, displayName string, systemAdmin bool, expectedVersion, actorID int64, audit *model.AuditLog) (*model.User, error) {
	if audit == nil || r.audits == nil {
		return nil, fmt.Errorf("managed user audit log is required")
	}
	var result *model.User
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		user, err := lockManagedUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		if expectedVersion < 0 || user.Version != expectedVersion {
			return ErrUserVersionConflict
		}
		if user.Enabled && user.IsSystemAdmin && !systemAdmin {
			if err := ensureLastAdminInTx(ctx, tx, user.ID); err != nil {
				return err
			}
		}
		updated, err := tx.Context(ctx).Exec(`
UPDATE users
SET display_name = ?, is_system_admin = ?, updated_by = ?,
    updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND version = ? AND deleted_at IS NULL`,
			displayName, systemAdmin, actorID, userID, expectedVersion)
		if err != nil {
			return fmt.Errorf("update managed user: %w", err)
		}
		if err := requireOneUser(updated); err != nil {
			return err
		}
		user.DisplayName = displayName
		user.IsSystemAdmin = systemAdmin
		user.UpdatedBy = actorID
		user.Version++
		audit.ResourceID = fmt.Sprintf("%d", userID)
		if err := r.audits.RecordSystemTx(ctx, tx, audit); err != nil {
			return err
		}
		result = user
		return nil
	})
	return result, err
}

func (r *UserRepository) UpdateManagedPassword(ctx context.Context, userID int64, passwordHash string, actorID int64, audit *model.AuditLog) error {
	if passwordHash == "" || audit == nil || r.audits == nil {
		return fmt.Errorf("managed password hash and audit log are required")
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		if _, err := lockManagedUser(ctx, tx, userID); err != nil {
			return err
		}
		updated, err := tx.Context(ctx).Exec(`
UPDATE users
SET password_hash = ?, updated_by = ?, updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND deleted_at IS NULL`, passwordHash, actorID, userID)
		if err != nil {
			return fmt.Errorf("update managed user password: %w", err)
		}
		if err := requireOneUser(updated); err != nil {
			return err
		}
		if _, err := tx.Context(ctx).Exec(
			"UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(6)), updated_at = UTC_TIMESTAMP(6) WHERE user_id = ?",
			userID,
		); err != nil {
			return fmt.Errorf("revoke password-reset sessions: %w", err)
		}
		audit.ResourceID = fmt.Sprintf("%d", userID)
		return r.audits.RecordSystemTx(ctx, tx, audit)
	})
}

func (r *UserRepository) UpdateManagedStatus(ctx context.Context, userID int64, enabled bool, expectedVersion, actorID int64, audit *model.AuditLog) (*model.User, error) {
	if audit == nil || r.audits == nil {
		return nil, fmt.Errorf("managed user audit log is required")
	}
	var result *model.User
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		user, err := lockManagedUser(ctx, tx, userID)
		if err != nil {
			return err
		}
		if expectedVersion < 0 || user.Version != expectedVersion {
			return ErrUserVersionConflict
		}
		if user.Enabled && user.IsSystemAdmin && !enabled {
			if err := ensureLastAdminInTx(ctx, tx, user.ID); err != nil {
				return err
			}
		}
		updated, err := tx.Context(ctx).Exec(`
UPDATE users
SET enabled = ?, updated_by = ?, updated_at = UTC_TIMESTAMP(6), version = version + 1
WHERE id = ? AND version = ? AND deleted_at IS NULL`, enabled, actorID, userID, expectedVersion)
		if err != nil {
			return fmt.Errorf("update managed user status: %w", err)
		}
		if err := requireOneUser(updated); err != nil {
			return err
		}
		if !enabled {
			if _, err := tx.Context(ctx).Exec(
				"UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, UTC_TIMESTAMP(6)), updated_at = UTC_TIMESTAMP(6) WHERE user_id = ?",
				userID,
			); err != nil {
				return fmt.Errorf("revoke disabled user sessions: %w", err)
			}
		}
		user.Enabled = enabled
		user.UpdatedBy = actorID
		user.Version++
		audit.ResourceID = fmt.Sprintf("%d", userID)
		if err := r.audits.RecordSystemTx(ctx, tx, audit); err != nil {
			return err
		}
		result = user
		return nil
	})
	return result, err
}

func lockManagedUser(ctx context.Context, tx *xorm.Session, userID int64) (*model.User, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	user := new(model.User)
	found, err := tx.Context(ctx).SQL(`
SELECT u.*
FROM users AS u
WHERE u.id = ? AND u.deleted_at IS NULL
LIMIT 1
FOR UPDATE`, userID).Get(user)
	if err != nil {
		return nil, fmt.Errorf("lock managed user: %w", err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func ensureLastAdminInTx(ctx context.Context, tx *xorm.Session, removingID int64) error {
	admins := make([]model.User, 0, 2)
	if err := tx.Context(ctx).SQL(`
SELECT u.*
FROM users AS u
WHERE u.enabled = TRUE AND u.is_system_admin = TRUE AND u.deleted_at IS NULL
ORDER BY u.id ASC
FOR UPDATE`).Find(&admins); err != nil {
		return fmt.Errorf("lock system administrators: %w", err)
	}
	return ensureSystemAdminSurvives(admins, removingID)
}

func requireOneUser(result interface{ RowsAffected() (int64, error) }) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read managed user update count: %w", err)
	}
	if affected != 1 {
		return ErrUserVersionConflict
	}
	return nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	user := new(model.User)
	session := r.engine.Context(ctx)
	defer session.Close()
	found, err := session.Where("username = ?", strings.TrimSpace(username)).Get(user)
	if err != nil {
		return nil, fmt.Errorf("find user by username: %w", err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, userID int64) (*model.User, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	user := new(model.User)
	session := r.engine.Context(ctx)
	defer session.Close()
	found, err := session.ID(userID).Get(user)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdatePasswordHash upgrades a successfully verified legacy hash without
// overwriting a password that another request changed concurrently.
func (r *UserRepository) UpdatePasswordHash(ctx context.Context, userID int64, previousHash, nextHash string) error {
	if userID <= 0 || previousHash == "" || nextHash == "" {
		return fmt.Errorf("password-hash update arguments are invalid")
	}
	session := r.engine.Context(ctx)
	defer session.Close()
	result, err := session.Exec(
		"UPDATE users SET password_hash = ?, updated_at = ?, version = version + 1 "+
			"WHERE id = ? AND password_hash = ? AND enabled = TRUE AND deleted_at IS NULL",
		nextHash, time.Now().UTC(), userID, previousHash,
	)
	if err != nil {
		return fmt.Errorf("upgrade user password hash: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read password-hash update count: %w", err)
	}
	if affected != 1 {
		return ErrUserVersionConflict
	}
	return nil
}

// ListEnvironments returns every enabled environment to a system
// administrator and only assigned enabled environments to ordinary users.
func (r *UserRepository) ListEnvironments(ctx context.Context, user *model.User) ([]model.Environment, error) {
	if user == nil || user.ID <= 0 {
		return nil, ErrUserNotFound
	}
	environments := make([]model.Environment, 0)
	session := r.engine.Context(ctx)
	defer session.Close()
	if user.IsSystemAdmin {
		if err := session.Where("enabled = ?", true).Asc("id").Find(&environments); err != nil {
			return nil, fmt.Errorf("list environments for system administrator: %w", err)
		}
		return environments, nil
	}
	err := session.Table("environments").Alias("e").
		Join("INNER", []string{"user_environment_roles", "uer"}, "uer.environment_id = e.id").
		Where("uer.user_id = ? AND e.enabled = ? AND e.deleted_at IS NULL", user.ID, true).
		Distinct("e.*").Asc("e.id").Find(&environments)
	if err != nil {
		return nil, fmt.Errorf("list assigned environments: %w", err)
	}
	return environments, nil
}

// CreateOrPromoteSystemAdmin creates a local user or safely restores and
// upgrades the existing username. It never creates database tables.
func (r *UserRepository) CreateOrPromoteSystemAdmin(ctx context.Context, username, displayName, passwordHash string) (*model.User, bool, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" || displayName == "" || passwordHash == "" {
		return nil, false, fmt.Errorf("username, display name, and password hash are required")
	}
	var result *model.User
	created := false
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		user := new(model.User)
		found, err := tx.Unscoped().Where("username = ?", username).ForUpdate().Get(user)
		if err != nil {
			return fmt.Errorf("lock administrator user: %w", err)
		}
		if !found {
			user.Username = username
			user.DisplayName = displayName
			user.PasswordHash = passwordHash
			user.Enabled = true
			user.IsSystemAdmin = true
			if _, err := tx.InsertOne(user); err != nil {
				return fmt.Errorf("create administrator user: %w", err)
			}
			created = true
			result = user
			return nil
		}

		update := "UPDATE users SET display_name = ?, password_hash = ?, enabled = TRUE, " +
			"is_system_admin = TRUE, deleted_at = NULL, updated_at = ?, version = version + 1 WHERE id = ?"
		if _, err := tx.Exec(update, displayName, passwordHash, time.Now().UTC(), user.ID); err != nil {
			return fmt.Errorf("promote administrator user: %w", err)
		}
		found, err = tx.Unscoped().ID(user.ID).Get(user)
		if err != nil {
			return fmt.Errorf("read promoted administrator user: %w", err)
		}
		if !found {
			return ErrUserNotFound
		}
		result = user
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return result, created, nil
}

func ensureSystemAdminSurvives(admins []model.User, removingID int64) error {
	remaining := 0
	for _, admin := range admins {
		if admin.ID != removingID && admin.Enabled && admin.IsSystemAdmin && admin.DeletedAt.IsZero() {
			remaining++
		}
	}
	if remaining == 0 {
		return ErrLastSystemAdmin
	}
	return nil
}
