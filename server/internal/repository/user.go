package repository

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

var (
	ErrUserNotFound        = errors.New("user not found")
	ErrLastSystemAdmin     = errors.New("cannot remove the last enabled system administrator")
	ErrUserVersionConflict = errors.New("user version conflict")
	ErrUserListLimit       = errors.New("user list exceeds fixed limit")
)

const managedUserListLimit = 1000

type ManagedUserUpdate struct {
	UserID      int64
	DisplayName string
	SystemAdmin bool
	Version     int64
	ActorID     int64
}

type ManagedUserStatusUpdate struct {
	UserID  int64
	Enabled bool
	Version int64
	ActorID int64
}

// UserRepository 读写本地管理员身份。
type UserRepository interface {
	ListManagedUsers(ctx context.Context) ([]model.User, error)
	CreateManagedUser(ctx context.Context, user *model.User, audit *model.AuditLog) error
	UpdateManagedUser(ctx context.Context, update ManagedUserUpdate, audit *model.AuditLog) (*model.User, error)
	UpdateManagedPassword(ctx context.Context, userID int64, passwordHash string, actorID int64, audit *model.AuditLog) error
	UpdateManagedStatus(ctx context.Context, update ManagedUserStatusUpdate, audit *model.AuditLog) (*model.User, error)
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	GetByID(ctx context.Context, userID int64) (*model.User, error)
	UpdatePasswordHash(ctx context.Context, userID int64, previousHash, nextHash string) error
	ListEnvironments(ctx context.Context, user *model.User) ([]model.Environment, error)
	CreateOrPromoteSystemAdmin(ctx context.Context, username, displayName, passwordHash string) (*model.User, bool, error)
}

type UserRepositoryImpl struct {
	*lorm.Repository[*model.User]
	audits AuditLogRepository
}

// NewUserRepository binds the UserRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured UserRepository.
// Throws: none.
func NewUserRepository(storage *infra.Storage) *UserRepositoryImpl {
	return &UserRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.User](),
		audits:     NewAuditLogRepository(storage),
	}
}

// ListManagedUsers returns active users in ascending ID order with a fixed management-list limit.
//
// Parameters: ctx controls the database read.
// Returns: active users; ErrUserListLimit when more than 1000 users exist, or the database error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) ListManagedUsers(ctx context.Context) ([]model.User, error) {
	fields := new(model.User).LormCols()
	users, err := r.Engine.Query[*model.User]().Where(builder.IsNull(fields.DeletedAt())).
		Asc(fields.ID()).Limit(managedUserListLimit + 1).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if len(users) > managedUserListLimit {
		return nil, ErrUserListLimit
	}
	return modelValues(users), nil
}

// CreateManagedUser creates an enabled account and records its mandatory audit in the same transaction.
//
// Parameters: ctx carries the transaction; user supplies account fields; audit records the actor and action.
// Returns: nil after both writes succeed, or a duplicate-username, database, or audit error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) CreateManagedUser(ctx context.Context, user *model.User, audit *model.AuditLog) error {
	user.PrepareInsert()
	user.Enabled = true
	if _, err := r.Insert(ctx, user); err != nil {
		return classifyKeyWriteError("user", err)
	}
	return r.recordAudit(ctx, user.ID, audit)
}

// UpdateManagedUser updates a matching user version without removing the final enabled administrator.
//
// Parameters: ctx carries the transaction; update supplies user ID, display name, administrator flag, expected version, and actor; audit records the change.
// Returns: the updated user, ErrUserNotFound, ErrUserVersionConflict, ErrLastSystemAdmin, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) UpdateManagedUser(ctx context.Context, update ManagedUserUpdate, audit *model.AuditLog) (*model.User, error) {
	fields := new(model.User).LormCols()
	user, err := lockManagedUser(ctx, r.Engine, update.UserID)
	if err != nil {
		return nil, err
	}
	if user.Version != update.Version {
		return nil, ErrUserVersionConflict
	}
	if user.Enabled && user.IsSystemAdmin && !update.SystemAdmin {
		if err := ensureLastSystemAdmin(ctx, r.Engine, user.ID); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	if err := r.updateManagedUser(ctx, update.UserID, map[string]any{
		fields.DisplayName():   update.DisplayName,
		fields.IsSystemAdmin(): update.SystemAdmin,
		fields.UpdatedBy():     update.ActorID,
		fields.UpdatedAt():     now,
		fields.Version():       update.Version + 1,
	}, update.Version); err != nil {
		return nil, err
	}
	user.DisplayName = update.DisplayName
	user.IsSystemAdmin = update.SystemAdmin
	user.UpdatedBy = update.ActorID
	user.UpdatedAt = now
	user.Version++
	if err := r.recordAudit(ctx, update.UserID, audit); err != nil {
		return nil, err
	}
	return user, nil
}

// UpdateManagedPassword changes a password and revokes all refresh sessions atomically.
//
// Parameters: ctx carries the transaction; userID identifies the account; passwordHash is the prepared hash; actorID identifies the editor; audit records the change.
// Returns: nil after the password, session revocations, and audit succeed, or a missing-user or persistence error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) UpdateManagedPassword(ctx context.Context, userID int64, passwordHash string, actorID int64, audit *model.AuditLog) error {
	fields := new(model.User).LormCols()
	user, err := lockManagedUser(ctx, r.Engine, userID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.PasswordHash():       passwordHash,
			fields.MustChangePassword(): false,
			fields.UpdatedBy():          actorID,
			fields.UpdatedAt():          now,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected != 1 {
		return ErrUserVersionConflict
	}
	if err := revokeUserSessions(ctx, r.Engine, user.ID, now); err != nil {
		return err
	}
	return r.recordAudit(ctx, userID, audit)
}

// UpdateManagedStatus changes a matching user version and revokes sessions when disabling the account.
//
// Parameters: ctx carries the transaction; update supplies user ID, enabled status, expected version, and actor; audit records the change.
// Returns: the updated user, a missing-user or version error, ErrLastSystemAdmin, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) UpdateManagedStatus(ctx context.Context, update ManagedUserStatusUpdate, audit *model.AuditLog) (*model.User, error) {
	fields := new(model.User).LormCols()
	user, err := lockManagedUser(ctx, r.Engine, update.UserID)
	if err != nil {
		return nil, err
	}
	if user.Version != update.Version {
		return nil, ErrUserVersionConflict
	}
	if user.Enabled && user.IsSystemAdmin && !update.Enabled {
		if err := ensureLastSystemAdmin(ctx, r.Engine, user.ID); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	if err := r.updateManagedUser(ctx, update.UserID, map[string]any{
		fields.Enabled():   update.Enabled,
		fields.UpdatedBy(): update.ActorID,
		fields.UpdatedAt(): now,
		fields.Version():   update.Version + 1,
	}, update.Version); err != nil {
		return nil, err
	}
	if !update.Enabled {
		if err := revokeUserSessions(ctx, r.Engine, user.ID, time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	user.Enabled = update.Enabled
	user.UpdatedBy = update.ActorID
	user.UpdatedAt = now
	user.Version++
	if err := r.recordAudit(ctx, update.UserID, audit); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepositoryImpl) updateManagedUser(ctx context.Context, userID int64, values map[string]any, expectedVersion int64) error {
	fields := new(model.User).LormCols()
	affected, err := r.Engine.Update[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).
		Where(builder.Eq{fields.Version(): expectedVersion}).
		SetMap(values).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected != 1 {
		return ErrUserVersionConflict
	}
	return nil
}

// revokeUserSessions 吊销用户全部未吊销的刷新令牌。
func revokeUserSessions(ctx context.Context, engine *lorm.Engine, userID int64, now time.Time) error {
	fields := new(model.RefreshToken).LormCols()
	_, err := engine.Update[*model.RefreshToken]().
		Where(builder.Eq{fields.UserID(): userID}).
		Where(builder.IsNull("revoked_at")).
		SetMap(map[string]any{
			fields.RevokedAt(): now,
			fields.UpdatedAt(): now,
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	return nil
}

func lockManagedUser(ctx context.Context, engine *lorm.Engine, userID int64) (*model.User, error) {
	user, found, err := engine.Query[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func ensureLastSystemAdmin(ctx context.Context, engine *lorm.Engine, removingID int64) error {
	fields := new(model.User).LormCols()
	admins, err := engine.Query[*model.User]().Where(builder.And{
		builder.Eq{
			fields.Enabled():       true,
			fields.IsSystemAdmin(): true,
		},
		builder.IsNull(fields.DeletedAt()),
	}).Asc(fields.ID()).Limit(2).Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	return ensureSystemAdminSurvives(modelValues(admins), removingID)
}

// FindByUsername excludes deleted accounts from credential lookup.
//
// Parameters: ctx controls the read; username is the normalized login name.
// Returns: the live account, ErrUserNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	fields := new(model.User).LormCols()
	user, found, err := r.Engine.Query[*model.User]().Where(builder.And{
		builder.Eq{
			fields.Username(): username,
		},
		builder.IsNull(fields.DeletedAt()),
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GetByID excludes deleted accounts so successful callers receive a usable record.
//
// Parameters: ctx controls the read; userID identifies the account.
// Returns: the live account, ErrUserNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) GetByID(ctx context.Context, userID int64) (*model.User, error) {
	user, found, err := r.Engine.Query[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdatePasswordHash guards a password-hash upgrade against concurrent account changes.
//
// Parameters: ctx controls the write; userID identifies the account; previousHash is the expected hash; nextHash is the replacement.
// Returns: nil after one enabled account changes, ErrUserVersionConflict, or a database error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) UpdatePasswordHash(ctx context.Context, userID int64, previousHash, nextHash string) error {
	fields := new(model.User).LormCols()
	affected, err := r.Engine.Update[*model.User]().ID(userID).
		Where(builder.IsNull(new(model.User).LormCols().DeletedAt())).
		Where(builder.Eq{
			fields.PasswordHash(): previousHash,
			fields.Enabled():      true,
		}).
		Set(fields.Version(), builder.Expr("version + 1")).
		SetMap(map[string]any{
			fields.PasswordHash(): nextHash,
			fields.UpdatedAt():    time.Now().UTC(),
		}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected != 1 {
		return ErrUserVersionConflict
	}
	return nil
}

// ListEnvironments returns enabled environments visible to the current account.
//
// Parameters: ctx controls the read; user is the current persisted account and determines administrator visibility.
// Returns: visible environments ordered by ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) ListEnvironments(ctx context.Context, user *model.User) ([]model.Environment, error) {
	query := r.Engine.Query[*model.Environment]().From("environments e").Select("e.*").
		Where(builder.And{
			builder.Eq{
				"e.enabled": true,
			},
			builder.IsNull("e.deleted_at"),
		}).Asc("e.id")
	if !user.IsSystemAdmin {
		query.InnerJoin("user_environment_roles uer ON uer.environment_id = e.id").
			Where(builder.Eq{"uer.user_id": user.ID}).Distinct()
	}
	environments, err := query.Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(environments), nil
}

// CreateOrPromoteSystemAdmin creates or restores a CLI administrator and revokes sessions when restoring an existing account.
//
// Parameters: ctx carries the provisioning transaction; username selects the account; displayName and passwordHash replace its credentials.
// Returns: the persisted administrator, whether it was newly inserted, and any write or session-revocation error.
// Throws: none; failures are returned as errors.
func (r *UserRepositoryImpl) CreateOrPromoteSystemAdmin(ctx context.Context, username, displayName, passwordHash string) (*model.User, bool, error) {
	user, found, err := r.Engine.Query[*model.User]().
		Where(builder.Eq{new(model.User).LormCols().Username(): username}).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, false, WrapDatabaseError(err)
	}
	if !found {
		created, err := r.createSystemAdmin(ctx, username, displayName, passwordHash)
		return created, true, err
	}
	if err := r.promoteSystemAdmin(ctx, user, displayName, passwordHash); err != nil {
		return nil, false, err
	}
	promoted, err := r.GetByID(ctx, user.ID)
	return promoted, false, err
}

func (r *UserRepositoryImpl) createSystemAdmin(ctx context.Context, username, displayName, passwordHash string) (*model.User, error) {
	user := &model.User{
		Username:           username,
		DisplayName:        displayName,
		PasswordHash:       passwordHash,
		Enabled:            true,
		IsSystemAdmin:      true,
		MustChangePassword: false,
	}
	user.PrepareInsert()
	if _, err := r.Insert(ctx, user); err != nil {
		return nil, classifyKeyWriteError("user", err)
	}
	return user, nil
}

func (r *UserRepositoryImpl) promoteSystemAdmin(ctx context.Context, user *model.User, displayName, passwordHash string) error {
	fields := new(model.User).LormCols()
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.User]().ID(user.ID).SetMap(map[string]any{
		fields.DisplayName():        displayName,
		fields.PasswordHash():       passwordHash,
		fields.Enabled():            true,
		fields.IsSystemAdmin():      true,
		fields.MustChangePassword(): false,
		fields.DeletedAt():          nil,
		fields.UpdatedAt():          now,
		fields.Version():            user.Version + 1,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("user")
	}
	return revokeUserSessions(ctx, r.Engine, user.ID, now)
}

func ensureSystemAdminSurvives(admins []model.User, removingID int64) error {
	remaining := 0
	for _, admin := range admins {
		if admin.ID != removingID && admin.Enabled && admin.IsSystemAdmin && admin.DeletedAt == nil {
			remaining++
		}
	}
	if remaining == 0 {
		return ErrLastSystemAdmin
	}
	return nil
}

func (r *UserRepositoryImpl) recordAudit(ctx context.Context, userID int64, audit *model.AuditLog) error {
	audit.ResourceID = strconv.FormatInt(userID, 10)
	return r.audits.RecordSystem(ctx, audit)
}

var _ UserRepository = (*UserRepositoryImpl)(nil)
