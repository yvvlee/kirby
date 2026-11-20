package logic

import (
	"context"
	"crypto/subtle"
	"strconv"
	"time"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type APIKeySecretResult struct {
	Key    *commonv1.ProjectApiKey
	Secret string
}

type APIKeyLogic struct {
	repository   repository.ProjectAPIKeyRepository
	credentials  *credential.Manager
	permissions  environmentPermissionChecker
	audits       repository.AuditLogRepository
	transactions TransactionRunner
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
	}
}

// List requires credential-read permission and removes stored digests from returned records.
//
// Parameters: ctx controls reads; actor identifies the requester; environmentID and projectID define ownership.
// Returns: public credential records, or an authorization or database error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) List(ctx context.Context, actor permission.Actor, environmentID, projectID int64) ([]*commonv1.ProjectApiKey, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyRead); err != nil {
		return nil, err
	}
	items, err := l.repository.List(ctx, environmentID, projectID)
	if err != nil {
		return nil, err
	}
	result := make([]*commonv1.ProjectApiKey, 0, len(items))
	for index := range items {
		items[index].SecretHash = nil
		result = append(result, converter.APIKeyToProto(&items[index]))
	}
	return result, nil
}

// Create returns a secret only after its digest and creation audit commit.
//
// Parameters: ctx controls the operation; param carries the caller identity, ownership scope, and credential label.
// Returns: the public credential and its one-time secret, or an authorization, generation, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Create(ctx context.Context, param *entity.CreateAPIKeyParam) (*APIKeySecretResult, error) {
	actor, environmentID, projectID := param.Actor, param.EnvironmentID, param.ProjectID
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return nil, err
	}
	generated, err := l.credentials.Generate()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			CreatedAt: now,
			UpdatedAt: now,
		},
		ProjectID:    projectID,
		Name:         param.Name,
		SecretHash:   append([]byte(nil), generated.Hash...),
		SecretSuffix: generated.SecretSuffix,
		CreatedBy:    actor.UserID,
	}
	if err := l.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := l.repository.Create(txctx, environmentID, projectID, item); err != nil {
			return err
		}
		_, err := l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.create", item.ID))
		return err
	}); err != nil {
		return nil, err
	}
	return &APIKeySecretResult{
		Key:    converter.APIKeyToProto(item),
		Secret: generated.Full,
	}, nil
}

// Rotate locks the scoped credential and compares its credential digest before replacing the secret.
//
// Parameters: ctx controls the operation; param carries the caller identity and the ownership scope identifying the credential.
// Returns: the rotated public credential and one-time secret, or an authorization, revocation, concurrent-change, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Rotate(ctx context.Context, param *entity.RotateAPIKeyParam) (*APIKeySecretResult, error) {
	actor, environmentID, projectID, keyID := param.Actor, param.EnvironmentID, param.ProjectID, param.KeyID
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
	now := time.Now().UTC()
	var result *model.ProjectAPIKey
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		locked, err := l.repository.LockByID(txctx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked.RevokedAt != nil {
			return entity.Conflict("revoked API key cannot be rotated")
		}
		if !sameSecretHash(locked.SecretHash, found.SecretHash) {
			return entity.Conflict("API key changed concurrently")
		}
		replacement := *locked
		replacement.SecretHash = append([]byte(nil), generated.Hash...)
		replacement.SecretSuffix = generated.SecretSuffix
		replacement.LastUsedAt = nil
		replacement.UpdatedAt = now
		if _, err := l.repository.Rotate(txctx, repository.APIKeyMutation{
			EnvironmentID:     environmentID,
			ProjectID:         projectID,
			KeyID:             keyID,
			CurrentSecretHash: locked.SecretHash,
		}, &replacement); err != nil {
			return err
		}
		if _, err := l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.rotate", keyID)); err != nil {
			return err
		}
		result = &replacement
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &APIKeySecretResult{
		Key:    converter.APIKeyToProto(result),
		Secret: generated.Full,
	}, nil
}

// Revoke locks the scoped credential and checks for concurrent rotation before revocation.
//
// Parameters: ctx controls the operation; param carries the caller identity and the ownership scope identifying the credential.
// Returns: nil after revocation and audit commit, or an authorization, concurrent-change, already-revoked, or transaction error.
// Throws: none; failures are returned as errors.
func (l *APIKeyLogic) Revoke(ctx context.Context, param *entity.RevokeAPIKeyParam) error {
	actor, environmentID, projectID, keyID := param.Actor, param.EnvironmentID, param.ProjectID, param.KeyID
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
	revokedAt := time.Now().UTC()
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		locked, err := l.repository.LockByID(txctx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked.RevokedAt != nil {
			return entity.Conflict("API key is already revoked")
		}
		if !sameSecretHash(locked.SecretHash, found.SecretHash) {
			return entity.Conflict("API key changed concurrently")
		}
		if _, err := l.repository.Revoke(txctx, repository.APIKeyMutation{
			EnvironmentID:     environmentID,
			ProjectID:         projectID,
			KeyID:             keyID,
			CurrentSecretHash: locked.SecretHash,
		}, revokedAt); err != nil {
			return err
		}
		_, err = l.audits.RecordForEnvironment(txctx, environmentID, apiKeyAudit(actor, "project_api_key.revoke", keyID))
		return err
	})
}

func sameSecretHash(left, right []byte) bool {
	return subtle.ConstantTimeCompare(left, right) == 1
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
