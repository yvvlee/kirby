package logic

import (
	"context"
	"fmt"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type userAuthorizer interface {
	RequireSystem(ctx context.Context, userID int64, required string) error
}

type userPasswordHasher interface {
	Hash(plainPassword string) (string, error)
}

type UserCreate struct {
	Username      string
	DisplayName   string
	PlainPassword string
	SystemAdmin   bool
}

type UserUpdate struct {
	UserID      int64
	DisplayName string
	SystemAdmin bool
	Version     int64
}

type UserLogic struct {
	users        repository.UserRepository
	permissions  userAuthorizer
	passwords    userPasswordHasher
	transactions TransactionRunner
}

// NewUserLogic constructs the UserLogic with explicit application dependencies.
//
// Parameters: users persists accounts; permissions enforces administrative access; passwords hashes credentials; transactions commits account changes and audits together.
// Returns: the configured UserLogic.
// Throws: none.
func NewUserLogic(
	users repository.UserRepository,
	permissions userAuthorizer,
	passwords userPasswordHasher,
	transactions TransactionRunner,
) *UserLogic {
	return &UserLogic{
		users:        users,
		permissions:  permissions,
		passwords:    passwords,
		transactions: transactions,
	}
}

// List limits account management data to system user managers.
//
// Parameters: ctx controls reads; actor identifies the requester.
// Returns: the bounded live-user list, or an authorization or repository error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) List(ctx context.Context, actor permission.Actor) ([]model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	return l.users.ListManagedUsers(ctx)
}

// Create hashes a new account password before committing the audited creation.
//
// Parameters: ctx controls the operation; actor identifies the creator; input supplies username, display name, password, and administrator status.
// Returns: the persisted user, or an authorization, hashing, or transaction error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) Create(ctx context.Context, actor permission.Actor, input UserCreate) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	hash, err := l.passwords.Hash(input.PlainPassword)
	if err != nil {
		return nil, fmt.Errorf("hash managed user password: %w", err)
	}
	user := &model.User{
		Username:      input.Username,
		DisplayName:   input.DisplayName,
		PasswordHash:  hash,
		Enabled:       true,
		IsSystemAdmin: input.SystemAdmin,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.users.CreateManagedUser(txctx, user, userAudit(actor, "user.create"))
	}); err != nil {
		return nil, err
	}
	return l.users.GetByID(ctx, user.ID)
}

// Update commits a version-guarded account edit under system user-management permission.
//
// Parameters: ctx controls the operation; actor identifies the editor; input supplies user ID, display name, administrator status, and expected version.
// Returns: the updated user, or an authorization, version, last-administrator, or transaction error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) Update(ctx context.Context, actor permission.Actor, input UserUpdate) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	var updated *model.User
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		updated, err = l.users.UpdateManagedUser(txctx, repository.ManagedUserUpdate{
			UserID:      input.UserID,
			DisplayName: input.DisplayName,
			SystemAdmin: input.SystemAdmin,
			Version:     input.Version,
			ActorID:     actor.UserID,
		}, userAudit(actor, "user.update"))
		return err
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

// UpdatePassword allows self-service password changes and requires system permission for other accounts.
//
// Parameters: ctx controls the operation; actor identifies the editor; userID selects the account; plainPassword is hashed before persistence.
// Returns: nil after the password, session revocations, and audit commit, or an authorization, hashing, or transaction error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) UpdatePassword(ctx context.Context, actor permission.Actor, userID int64, plainPassword string) error {
	if actor.UserID != userID {
		if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
			return err
		}
	}
	hash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return fmt.Errorf("hash managed user password: %w", err)
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.users.UpdateManagedPassword(txctx, userID, hash, actor.UserID, userAudit(actor, "user.password.update"))
	})
}

// UpdateStatus requires system user-management permission before changing account availability.
//
// Parameters: ctx controls the operation; actor identifies the editor; userID selects the account; enabled is the desired status; version is the expected version.
// Returns: the updated user, or an authorization, version, last-administrator, or transaction error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) UpdateStatus(ctx context.Context, actor permission.Actor, userID int64, enabled bool, version int64) (*model.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	var updated *model.User
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		updated, err = l.users.UpdateManagedStatus(txctx, repository.ManagedUserStatusUpdate{
			UserID:  userID,
			Enabled: enabled,
			Version: version,
			ActorID: actor.UserID,
		}, userAudit(actor, "user.status.update"))
		return err
	}); err != nil {
		return nil, err
	}
	return updated, nil
}

func userAudit(actor permission.Actor, action string) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "user",
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
