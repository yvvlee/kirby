package api_key

import (
	"context"
	"crypto/subtle"
	"fmt"
	"strconv"
	"time"

	"xorm.io/xorm"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

type Repository interface {
	CreateTx(context.Context, *xorm.Session, int64, int64, *model.ProjectAPIKey) error
	List(context.Context, int64, int64) ([]model.ProjectAPIKey, error)
	FindByID(context.Context, int64, int64, int64) (*model.ProjectAPIKey, error)
	LockByID(context.Context, *xorm.Session, int64, int64, int64) (*model.ProjectAPIKey, error)
	RotateTx(context.Context, *xorm.Session, int64, int64, int64, string, *model.ProjectAPIKey) error
	RevokeTx(context.Context, *xorm.Session, int64, int64, int64, string, time.Time) error
}

type Authorizer interface {
	Require(context.Context, int64, int64, ...string) error
}

type AuditRepository interface {
	RecordForEnvironmentTx(context.Context, *xorm.Session, int64, *model.AuditLog) error
}

type SecretResult struct {
	Key    *model.ProjectAPIKey
	Secret string
}

type Logic struct {
	repository   Repository
	credentials  *credential.Manager
	permissions  Authorizer
	audits       AuditRepository
	transactions database.Transactor
	now          func() time.Time
}

func New(repository Repository, credentials *credential.Manager, permissions Authorizer, audits AuditRepository, transactions database.Transactor) (*Logic, error) {
	if repository == nil || credentials == nil || permissions == nil || audits == nil || transactions == nil {
		return nil, fmt.Errorf("project API key logic dependencies are incomplete")
	}
	return &Logic{repository: repository, credentials: credentials, permissions: permissions, audits: audits, transactions: transactions, now: time.Now}, nil
}

func (l *Logic) List(ctx context.Context, actor permission.Actor, environmentID, projectID int64) ([]model.ProjectAPIKey, error) {
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

func (l *Logic) Create(ctx context.Context, actor permission.Actor, environmentID, projectID int64, name string) (*SecretResult, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return nil, err
	}
	generated, err := l.credentials.Generate()
	if err != nil {
		return nil, err
	}
	now := l.now().UTC()
	item := &model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{CreatedAt: now, UpdatedAt: now},
		ProjectID:  projectID, PublicID: generated.PublicID, Name: name,
		SecretHash: append([]byte(nil), generated.Hash...), SecretSuffix: generated.SecretSuffix,
		CreatedBy: actor.UserID,
	}
	if err := l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		if err := l.repository.CreateTx(ctx, tx, environmentID, projectID, item); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "project_api_key.create", item.ID))
	}); err != nil {
		return nil, err
	}
	return &SecretResult{Key: publicKey(item), Secret: generated.Full}, nil
}

func (l *Logic) Rotate(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) (*SecretResult, error) {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return nil, err
	}
	found, err := l.repository.FindByID(ctx, environmentID, projectID, keyID)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("project API key repository returned nil key")
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
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		locked, err := l.repository.LockByID(ctx, tx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked == nil {
			return fmt.Errorf("project API key repository returned nil locked key")
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
		if err := l.repository.RotateTx(ctx, tx, environmentID, projectID, keyID, locked.PublicID, &replacement); err != nil {
			return err
		}
		if err := l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "project_api_key.rotate", keyID)); err != nil {
			return err
		}
		result = &replacement
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &SecretResult{Key: publicKey(result), Secret: generated.Full}, nil
}

func (l *Logic) Revoke(ctx context.Context, actor permission.Actor, environmentID, projectID, keyID int64) error {
	if err := l.permissions.Require(ctx, actor.UserID, environmentID, permission.ProjectAPIKeyManage); err != nil {
		return err
	}
	found, err := l.repository.FindByID(ctx, environmentID, projectID, keyID)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("project API key repository returned nil key")
	}
	if found.RevokedAt != nil {
		return entity.Conflict("API key is already revoked")
	}
	revokedAt := l.now().UTC()
	return l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		locked, err := l.repository.LockByID(ctx, tx, environmentID, projectID, keyID)
		if err != nil {
			return err
		}
		if locked == nil {
			return fmt.Errorf("project API key repository returned nil locked key")
		}
		if locked.RevokedAt != nil {
			return entity.Conflict("API key is already revoked")
		}
		if !samePublicID(locked.PublicID, found.PublicID) {
			return entity.Conflict("API key changed concurrently")
		}
		if err := l.repository.RevokeTx(ctx, tx, environmentID, projectID, keyID, locked.PublicID, revokedAt); err != nil {
			return err
		}
		return l.audits.RecordForEnvironmentTx(ctx, tx, environmentID, audit(actor, "project_api_key.revoke", keyID))
	})
}

func samePublicID(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func publicKey(item *model.ProjectAPIKey) *model.ProjectAPIKey {
	if item == nil {
		return nil
	}
	result := *item
	result.SecretHash = nil
	return &result
}

func audit(actor permission.Actor, action string, keyID int64) *model.AuditLog {
	actorID := actor.UserID
	return &model.AuditLog{
		ActorUserID: &actorID, Action: action, ResourceType: "project_api_key",
		ResourceID: strconv.FormatInt(keyID, 10), Result: model.AuditResultSucceeded,
		RequestID: actor.RequestID,
	}
}
