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
)

// UserRepository reads and mutates local administrator identities.
type UserRepository struct {
	engine *xorm.Engine
}

func NewUserRepository(engine *xorm.Engine) (*UserRepository, error) {
	if engine == nil {
		return nil, fmt.Errorf("user repository database is nil")
	}
	return &UserRepository{engine: engine}, nil
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

// UpdateSecurityState atomically changes global user status. Locking all
// currently usable system administrators prevents concurrent requests from
// each removing a different "last" administrator.
func (r *UserRepository) UpdateSecurityState(ctx context.Context, userID int64, enabled, systemAdmin bool, expectedVersion, actorID int64) (*model.User, error) {
	var result *model.User
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		user := new(model.User)
		found, err := tx.ID(userID).ForUpdate().Get(user)
		if err != nil {
			return fmt.Errorf("lock user security state: %w", err)
		}
		if !found {
			return ErrUserNotFound
		}
		if expectedVersion < 0 || user.Version != expectedVersion {
			return ErrUserVersionConflict
		}
		if user.Enabled && user.IsSystemAdmin && (!enabled || !systemAdmin) {
			admins := make([]model.User, 0, 2)
			if err := tx.Where("enabled = ? AND is_system_admin = ?", true, true).ForUpdate().Find(&admins); err != nil {
				return fmt.Errorf("lock system administrators: %w", err)
			}
			if err := ensureSystemAdminSurvives(admins, user.ID); err != nil {
				return err
			}
		}
		resultSet, err := tx.Exec(
			"UPDATE users SET enabled = ?, is_system_admin = ?, updated_by = ?, updated_at = ?, version = version + 1 "+
				"WHERE id = ? AND version = ? AND deleted_at IS NULL",
			enabled, systemAdmin, actorID, time.Now().UTC(), user.ID, expectedVersion,
		)
		if err != nil {
			return fmt.Errorf("update user security state: %w", err)
		}
		affected, err := resultSet.RowsAffected()
		if err != nil {
			return fmt.Errorf("read updated user count: %w", err)
		}
		if affected != 1 {
			return ErrUserVersionConflict
		}
		user.Enabled = enabled
		user.IsSystemAdmin = systemAdmin
		user.UpdatedBy = actorID
		user.Version++
		result = user
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
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
