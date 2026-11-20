package logic

import (
	"context"
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
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
// Returns: the bounded live-user list, or an authorization, repository, or conversion error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) List(ctx context.Context, actor permission.Actor) ([]*commonv1.User, error) {
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	items, err := l.users.ListManagedUsers(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*commonv1.User, 0, len(items))
	for index := range items {
		converted, err := converter.UserToProto(&items[index])
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

// Create hashes a new account password before committing the audited creation.
//
// Parameters: ctx controls the operation; param carries the caller identity and the requested account fields including the plain password.
// Returns: the persisted user, or an authorization, hashing, transaction, or conversion error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) Create(ctx context.Context, param *entity.UserCreateParam) (*commonv1.User, error) {
	actor := param.Actor
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	hash, err := l.passwords.Hash(param.PlainPassword)
	if err != nil {
		return nil, fmt.Errorf("hash managed user password: %w", err)
	}
	user := &model.User{
		Username:      param.Username,
		Email:         param.Email,
		Nickname:      param.Nickname,
		PasswordHash:  hash,
		Enabled:       true,
		IsSystemAdmin: param.SystemAdmin,
		Meta: model.Meta{
			CreatedBy: actor.UserID,
			UpdatedBy: actor.UserID,
		},
	}
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		_, err := l.users.CreateManagedUser(txctx, user, userAudit(actor, "user.create"))
		return err
	}); err != nil {
		return nil, err
	}
	created, err := l.users.GetByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return converter.UserToProto(created)
}

// Update commits a version-guarded account edit under system user-management permission.
//
// Parameters: ctx controls the operation; param carries the caller identity, target account, and expected version.
// Returns: the updated user, or an authorization, version, last-administrator, transaction, or conversion error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) Update(ctx context.Context, param *entity.UserUpdateParam) (*commonv1.User, error) {
	actor := param.Actor
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	var updated *model.User
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		updated, err = l.users.UpdateManagedUser(txctx, repository.ManagedUserUpdate{
			UserID:      param.UserID,
			Nickname:    param.Nickname,
			SystemAdmin: param.SystemAdmin,
			Version:     param.Version,
			ActorID:     actor.UserID,
		}, userAudit(actor, "user.update"))
		return err
	}); err != nil {
		return nil, err
	}
	return converter.UserToProto(updated)
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
		_, err := l.users.UpdateManagedPassword(txctx, userID, hash, actor.UserID, userAudit(actor, "user.password.update"))
		return err
	})
}

// UpdateStatus requires system user-management permission before changing account availability.
//
// Parameters: ctx controls the operation; param carries the caller identity, target account, desired status, and expected version.
// Returns: the updated user, or an authorization, version, last-administrator, transaction, or conversion error.
// Throws: none; failures are returned as errors.
func (l *UserLogic) UpdateStatus(ctx context.Context, param *entity.UpdateUserStatusParam) (*commonv1.User, error) {
	actor := param.Actor
	if err := l.permissions.RequireSystem(ctx, actor.UserID, permission.SystemUserManage); err != nil {
		return nil, err
	}
	var updated *model.User
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		updated, err = l.users.UpdateManagedStatus(txctx, repository.ManagedUserStatusUpdate{
			UserID:  param.UserID,
			Enabled: param.Enabled,
			Version: param.Version,
			ActorID: actor.UserID,
		}, userAudit(actor, "user.status.update"))
		return err
	}); err != nil {
		return nil, err
	}
	return converter.UserToProto(updated)
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
