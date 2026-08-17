package user

import (
	"context"
	"fmt"
	"strings"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type Repository interface {
	ListManagedUsers(context.Context) ([]model.User, error)
	GetByID(context.Context, int64) (*model.User, error)
	CreateManagedUser(context.Context, *model.User, *model.AuditLog) error
	UpdateManagedUser(context.Context, int64, string, bool, int64, int64, *model.AuditLog) (*model.User, error)
	UpdateManagedPassword(context.Context, int64, string, int64, *model.AuditLog) error
	UpdateManagedStatus(context.Context, int64, bool, int64, int64, *model.AuditLog) (*model.User, error)
}

type Authorizer interface {
	RequireSystem(context.Context, int64, string) error
}

type PasswordHasher interface {
	Hash(string) (string, error)
}

type Logic struct {
	users       Repository
	permissions Authorizer
	passwords   PasswordHasher
}

func New(users Repository, permissions Authorizer, passwords PasswordHasher) (*Logic, error) {
	if users == nil || permissions == nil || passwords == nil {
		return nil, fmt.Errorf("user logic dependencies are incomplete")
	}
	return &Logic{users: users, permissions: permissions, passwords: passwords}, nil
}

func (l *Logic) List(ctx context.Context, actor permission.Actor) ([]model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	return l.users.ListManagedUsers(ctx)
}

func (l *Logic) Create(ctx context.Context, actor permission.Actor, username, displayName, plainPassword string, systemAdmin bool) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if username == "" || displayName == "" {
		return nil, base.InvalidArgument("username and display name are required")
	}
	hash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("hash managed user password: %w", err)
	}
	user := &model.User{
		Username: username, DisplayName: displayName, PasswordHash: hash, Enabled: true,
		IsSystemAdmin: systemAdmin, Meta: model.Meta{CreatedBy: actor.UserID, UpdatedBy: actor.UserID},
	}
	if err := l.users.CreateManagedUser(ctx, user, audit(actor, "user.create")); err != nil {
		return nil, err
	}
	return l.users.GetByID(ctx, user.ID)
}

func (l *Logic) Update(ctx context.Context, actor permission.Actor, userID int64, displayName string, systemAdmin bool, version int64) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		return nil, base.InvalidArgument("display name is required")
	}
	return l.users.UpdateManagedUser(ctx, userID, displayName, systemAdmin, version, actor.UserID, audit(actor, "user.update"))
}

func (l *Logic) UpdatePassword(ctx context.Context, actor permission.Actor, userID int64, plainPassword string) error {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return err
	}
	hash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return fmt.Errorf("hash managed user password: %w", err)
	}
	return l.users.UpdateManagedPassword(ctx, userID, hash, actor.UserID, audit(actor, "user.password.update"))
}

func (l *Logic) UpdateStatus(ctx context.Context, actor permission.Actor, userID int64, enabled bool, version int64) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	return l.users.UpdateManagedStatus(ctx, userID, enabled, version, actor.UserID, audit(actor, "user.status.update"))
}

func audit(actor permission.Actor, action string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID: &actorID, Action: action, ResourceType: "user",
		Result: model.AuditResultSucceeded, RequestID: actor.RequestID,
	}
}
