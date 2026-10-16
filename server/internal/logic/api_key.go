package logic

import (
	"context"
	"crypto/subtle"
	"strconv"
	"time"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type APIKeySecretResult struct {
	Key    *model.ProjectAPIKey
	Secret string
}

type APIKeyLogic struct {
	repository   repository.ProjectAPIKeyRepository
	credentials  *credential.Manager
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
	now          func() time.Time
}

// NewAPIKeyLogic constructs the APIKeyLogic with explicit application dependencies.
//
// Parameters: repository persists scoped credentials; credentials generates secrets; permissions authorizes access; audits records changes; transactions commits writes and audit together.
// Returns: the configured APIKeyLogic.
// Throws: none.
func NewAPIKeyLogic(
	repository repository.ProjectAPIKeyRepository,
	credentials *credential.Manager,
	permissions environmentPermissionChecker,
	audits repository.AuditLogRepository,
	transactions TransactionRunner,
) *APIKeyLogic {
	return &APIKeyLogic{
		repository:   repository,
		credentials:  credentials,
		permissions:  permissions,
		audits:       audits,
		transactions: transactions,
		now:          time.Now,
	}
}

// List requires credential-read permission and removes stored digests from returned records.
//
// Parameters: ctx controls reads; actor identifies the requester; environmentID and projectID define ownership.
// Returns: public credential records, or an authorization or database error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) List(ctx context.Context, actor permission.Actor, environmentID, projectID int64) ([]model.ProjectAPIKey, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyRead); err != nil {
		return nil, err
	}
	items, err := l.repository.List(ctx, environmentID, projectID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].SecretHash = nil
	}
	return items, nil
}

// Create returns a secret only after its digest and creation audit commit.
//
// Parameters: ctx controls the operation; actor identifies the creator; environmentID and projectID define ownership; name labels the credential.
// Returns: the public credential and its one-time secret, or an authorization, generation, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Create(ctx context.Context, actor permission.Actor, environmentID, projectID int64, name string) (*APIKeySecretResult, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return nil, err
	}
	generated, err := l.credentials.Generate()
	if err != nil {
		return nil, err
	}
	now := l.now().UTC()
	item := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			CreatedAt: now,
			UpdatedAt: now,
		},
		ProjectID:    projectID,
		PublicID:     generated.PublicID,
		Name:         name,
		SecretHash:   append([]byte(nil), generated.Hash...),
		SecretSuffix: generated.SecretSuffix,
		CreatedBy:    actor.UserID,
	}
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if err := l.repository.Create(txctx, environmentID, projectID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.create", item.ID))
	}); err != nil {
		return nil, err
	}
	return &APIKeySecretResult{
		Key:    publicKey(item),
		Secret: generated.Full,
	}, nil
}

// Rotate locks the scoped credential and compares its public ID before replacing the secret.
//
// Parameters: ctx controls the operation; actor identifies the editor; environmentID, projectID, and keyID identify the credential.
// Returns: the rotated public credential and one-time secret, or an authorization, revocation, concurrent-change, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Rotate(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) (*APIKeySecretResult, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return nil, err
	}
	found, err := l.repository.FindByID(ctx, environmentID, projectID, keyID)
	if err != nil {
		return nil, err
	}
	if found.RevokedAt != nil {
		return nil, entity.Conflict("revoked API key cannot be rotated")
	}
	generated, err := l.credentials.Generate()
	if err != nil {
		return nil, err
	}
	now := l.now().UTC()
	var result *model.ProjectAPIKey
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		locked, err := l.repository.LockByID(txctx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked.RevokedAt != nil {
			return entity.Conflict("revoked API key cannot be rotated")
		}
		if !samePublicID(locked.PublicID, found.PublicID) {
			return entity.Conflict("API key changed concurrently")
		}
		replacement := *locked
		replacement.PublicID = generated.PublicID
		replacement.SecretHash = append([]byte(nil), generated.Hash...)
		replacement.SecretSuffix = generated.SecretSuffix
		replacement.LastUsedAt = nil
		replacement.UpdatedAt = now
		if err := l.repository.Rotate(txctx, repository.APIKeyMutation{
			EnvironmentID:   environmentID,
			ProjectID:       projectID,
			KeyID:           keyID,
			CurrentPublicID: locked.PublicID,
		}, &replacement); err != nil {
			return err
		}
		if err := l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.rotate", keyID)); err != nil {
			return err
		}
		result = &replacement
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &APIKeySecretResult{
		Key:    publicKey(result),
		Secret: generated.Full,
	}, nil
}

// Revoke locks the scoped credential and checks for concurrent rotation before revocation.
//
// Parameters: ctx controls the operation; actor identifies the editor; environmentID, projectID, and keyID identify the credential.
// Returns: nil after revocation and audit commit, or an authorization, concurrent-change, already-revoked, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Revoke(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return err
	}
	found, err := l.repository.FindByID(ctx, environmentID, projectID, keyID)
	if err != nil {
		return err
	}
	if found.RevokedAt != nil {
		return entity.Conflict("API key is already revoked")
	}
	revokedAt := l.now().UTC()
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		locked, err := l.repository.LockByID(txctx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked.RevokedAt != nil {
			return entity.Conflict("API key is already revoked")
		}
		if !samePublicID(locked.PublicID, found.PublicID) {
			return entity.Conflict("API key changed concurrently")
		}
		if err := l.repository.Revoke(txctx, repository.APIKeyMutation{
			EnvironmentID:   environmentID,
			ProjectID:       projectID,
			KeyID:           keyID,
			CurrentPublicID: locked.PublicID,
		}, revokedAt); err != nil {
			return err
		}
		return l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.revoke", keyID))
	})
}

func samePublicID(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func publicKey(item *model.ProjectAPIKey) *model.ProjectAPIKey {
	result := *item
	result.SecretHash = nil
	return &result
}

func apiKeyAudit(actor permission.Actor, action string, keyID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID:  &actorID,
		Action:       action,
		ResourceType: "project_api_key",
		ResourceID:   strconv.FormatInt(keyID, 10),
		Result:       model.AuditResultSucceeded,
		RequestID:    actor.RequestID,
	}
}
