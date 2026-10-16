package repository

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

var (
	ErrRoleInUse             = errors.New("role is assigned to environment members")
	ErrBuiltinRole           = errors.New("built-in role cannot be deleted")
	ErrSystemRolePermission  = errors.New("system permission cannot be assigned to an environment role")
	ErrPermissionSetMismatch = errors.New("permission set contains unknown permission")
)

type RoleUpdate struct {
	Name        string
	Description string
	UpdatedBy   int64
	Version     int64
}

type RoleWithPermissions struct {
	Role        model.Role
	Permissions []model.Permission
}

type RoleRepository interface {
	List(ctx context.Context) ([]RoleWithPermissions, error)
	FindByID(ctx context.Context, roleID int64) (*RoleWithPermissions, error)
	Create(ctx context.Context, role *model.Role, audit *model.AuditLog) error
	Update(ctx context.Context, roleID int64, update RoleUpdate, audit *model.AuditLog) error
	Delete(ctx context.Context, roleID, actorID int64, audit *model.AuditLog) error
	UpdatePermissions(ctx context.Context, roleID int64, permissionIDs []int64, actorID int64, audit *model.AuditLog) ([]PermissionAssignment, error)
}

type RoleRepositoryImpl struct {
	*lorm.Repository[*model.Role]
	audits AuditLogRepository
}

// NewRoleRepository binds the RoleRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured RoleRepository.
// Throws: none.
func NewRoleRepository(storage *infra.Storage) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.Role](),
		audits:     NewAuditLogRepository(storage),
	}
}

// Create creates a custom role and records its mandatory audit atomically.
//
// Parameters: ctx carries the transaction; role supplies the new fields; audit records the actor and creation.
// Returns: nil after insertion and audit, or a duplicate-key or persistence error.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) Create(ctx context.Context, role *model.Role, audit *model.AuditLog) error {
	role.PrepareInsert()
	role.Builtin = false
	if _, err := r.Insert(ctx, role); err != nil {
		return classifyKeyWriteError("role", err)
	}
	return r.recordAudit(ctx, role.ID, audit)
}

// Update edits a live role only when its persisted version matches the request.
//
// Parameters: ctx carries the transaction; roleID selects the role; update supplies fields, actor, and expected version; audit records the edit.
// Returns: nil after update and audit, ErrNoRowsAffected for a missing or stale role, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) Update(ctx context.Context, roleID int64, update RoleUpdate, audit *model.AuditLog) error {
	fields := new(model.Role).LormCols()
	affected, err := r.Engine.Update[*model.Role]().ID(roleID).
		Where(builder.And{
			builder.Eq{
				fields.Version(): update.Version,
			},
			builder.IsNull(fields.DeletedAt()),
		}).SetMap(map[string]any{
		fields.Name():        update.Name,
		fields.Description(): update.Description,
		fields.UpdatedBy():   update.UpdatedBy,
		fields.UpdatedAt():   time.Now().UTC(),
		fields.Version():     update.Version + 1,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("role")
	}
	return r.recordAudit(ctx, roleID, audit)
}

// Delete soft-deletes an unassigned custom role while protecting built-in roles.
//
// Parameters: ctx carries the transaction; roleID selects the role; actorID identifies the editor; audit records deletion.
// Returns: nil after deletion and audit, ErrBuiltinRole, ErrRoleInUse, ErrNotFound, or a persistence error.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) Delete(ctx context.Context, roleID, actorID int64, audit *model.AuditLog) error {
	role, err := r.lockRole(ctx, roleID)
	if err != nil {
		return err
	}
	if role.Builtin {
		return ErrBuiltinRole
	}
	assignments, err := r.lockAssignments(ctx, roleID)
	if err != nil {
		return err
	}
	if len(assignments) > 0 {
		return ErrRoleInUse
	}
	fields := new(model.Role).LormCols()
	now := time.Now().UTC()
	affected, err := r.Engine.Update[*model.Role]().ID(roleID).
		Where(builder.IsNull(fields.DeletedAt())).SetMap(map[string]any{
		fields.Version():   builder.Expr("version + 1"),
		fields.DeletedAt(): now,
		fields.UpdatedBy(): actorID,
		fields.UpdatedAt(): now,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("role")
	}
	return r.recordAudit(ctx, roleID, audit)
}

// List returns live roles together with their current permission catalog entries.
//
// Parameters: ctx controls the role and permission reads.
// Returns: roles ordered by ID with permissions ordered by ID, or a database error.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) List(ctx context.Context) ([]RoleWithPermissions, error) {
	fields := new(model.Role).LormCols()
	roles, err := r.Engine.Query[*model.Role]().Where(builder.IsNull(fields.DeletedAt())).Asc(fields.ID()).Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return rolesWithPermissions(ctx, r.Engine, roles)
}

// FindByID requires a live role before reading its assigned permissions.
//
// Parameters: ctx controls the reads; roleID selects the role.
// Returns: the role and its permissions, ErrNotFound, or a database error.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) FindByID(ctx context.Context, roleID int64) (*RoleWithPermissions, error) {
	role, found, err := r.Engine.Query[*model.Role]().ID(roleID).
		Where(builder.IsNull(new(model.Role).LormCols().DeletedAt())).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("role")
	}
	permissions, err := permissionsForRole(ctx, r.Engine, roleID)
	if err != nil {
		return nil, err
	}
	return &RoleWithPermissions{
		Role:        *role,
		Permissions: permissions,
	}, nil
}

func rolesWithPermissions(ctx context.Context, engine *lorm.Engine, roles []*model.Role) ([]RoleWithPermissions, error) {
	result := make([]RoleWithPermissions, 0, len(roles))
	for _, role := range roles {
		permissions, err := permissionsForRole(ctx, engine, role.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, RoleWithPermissions{
			Role:        *role,
			Permissions: permissions,
		})
	}
	return result, nil
}

func permissionsForRole(ctx context.Context, engine *lorm.Engine, roleID int64) ([]model.Permission, error) {
	permissions, err := engine.Query[*model.Permission]().From("permissions p").Select("p.*").
		InnerJoin("role_permissions rp ON rp.permission_id = p.id").
		Where(builder.Eq{"rp.role_id": roleID}).Asc("p.id").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	return modelValues(permissions), nil
}

// UpdatePermissions replaces grants and advances every affected environment generation in one transaction.
//
// Parameters: ctx carries the transaction; roleID selects the role; permissionIDs is the complete replacement set; actorID identifies the editor; audit records the change.
// Returns: affected user/environment assignments; errors reject unknown or system-only permissions and failed writes.
// Throws: none; failures are returned as errors.
func (r *RoleRepositoryImpl) UpdatePermissions(ctx context.Context, roleID int64, permissionIDs []int64, actorID int64, audit *model.AuditLog) ([]PermissionAssignment, error) {
	if _, err := r.lockRole(ctx, roleID); err != nil {
		return nil, err
	}
	if err := r.lockPermissions(ctx, permissionIDs); err != nil {
		return nil, err
	}
	assignments, err := r.lockAssignments(ctx, roleID)
	if err != nil {
		return nil, err
	}
	if err := r.replacePermissions(ctx, roleID, permissionIDs, actorID); err != nil {
		return nil, err
	}
	if err := r.advancePermissionGenerations(ctx, roleID, actorID, assignments); err != nil {
		return nil, err
	}
	if err := r.recordAudit(ctx, roleID, audit); err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *RoleRepositoryImpl) lockRole(ctx context.Context, roleID int64) (*model.Role, error) {
	role, found, err := r.Engine.Query[*model.Role]().ID(roleID).
		Where(builder.IsNull(new(model.Role).LormCols().DeletedAt())).Suffix("FOR UPDATE").Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("role")
	}
	return role, nil
}

func (r *RoleRepositoryImpl) lockPermissions(ctx context.Context, permissionIDs []int64) error {
	if len(permissionIDs) == 0 {
		return nil
	}
	fields := new(model.Permission).LormCols()
	permissions, err := r.Engine.Query[*model.Permission]().Where(builder.In(fields.ID(), permissionIDs)).
		Asc(fields.ID()).Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if len(permissions) != len(permissionIDs) {
		return ErrPermissionSetMismatch
	}
	for _, item := range permissions {
		if strings.HasPrefix(item.Key, "system:") {
			return ErrSystemRolePermission
		}
	}
	return nil
}

func (r *RoleRepositoryImpl) lockAssignments(ctx context.Context, roleID int64) ([]PermissionAssignment, error) {
	fields := new(model.UserEnvironmentRole).LormCols()
	links, err := r.Engine.Query[*model.UserEnvironmentRole]().
		Where(builder.Eq{fields.RoleID(): roleID}).Asc(fields.ID()).Suffix("FOR UPDATE").Find(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	assignments := make([]PermissionAssignment, 0, len(links))
	for _, link := range links {
		assignments = append(assignments, PermissionAssignment{
			UserID:        link.UserID,
			EnvironmentID: link.EnvironmentID,
		})
	}
	return assignments, nil
}

func (r *RoleRepositoryImpl) replacePermissions(ctx context.Context, roleID int64, permissionIDs []int64, actorID int64) error {
	if _, err := r.Engine.Delete[*model.RolePermission]().
		Where(builder.Eq{new(model.RolePermission).LormCols().RoleID(): roleID}).Exec(ctx); err != nil {
		return WrapDatabaseError(err)
	}
	if len(permissionIDs) == 0 {
		return nil
	}
	links := make([]*model.RolePermission, 0, len(permissionIDs))
	for _, permissionID := range permissionIDs {
		link := &model.RolePermission{
			RoleID:       roleID,
			PermissionID: permissionID,
			CreatedBy:    actorID,
		}
		link.PrepareInsert()
		links = append(links, link)
	}
	_, err := r.Engine.Insert[*model.RolePermission]().AddModels(links...).Exec(ctx)
	return WrapDatabaseError(err)
}

func (r *RoleRepositoryImpl) advancePermissionGenerations(ctx context.Context, roleID, actorID int64, assignments []PermissionAssignment) error {
	fields := new(model.Role).LormCols()
	affected, err := r.Engine.Update[*model.Role]().ID(roleID).
		Where(builder.IsNull(fields.DeletedAt())).SetMap(map[string]any{
		fields.Version():   builder.Expr("version + 1"),
		fields.UpdatedBy(): actorID,
		fields.UpdatedAt(): time.Now().UTC(),
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected == 0 {
		return Unchanged("role")
	}
	environmentIDs := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		environmentIDs = append(environmentIDs, assignment.EnvironmentID)
	}
	return bumpEnvironmentPermissionGenerations(ctx, r.Engine, environmentIDs, actorID)
}

func (r *RoleRepositoryImpl) recordAudit(ctx context.Context, roleID int64, audit *model.AuditLog) error {
	audit.ResourceID = strconv.FormatInt(roleID, 10)
	return r.audits.RecordSystem(ctx, audit)
}

var _ RoleRepository = (*RoleRepositoryImpl)(nil)
