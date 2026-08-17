package runtime

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"xorm.io/xorm"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

const contentTTL = 24 * time.Hour

var (
	ErrUnauthenticated = errors.New("runtime API key authentication failed")
	ErrProjectMismatch = errors.New("runtime API key does not belong to requested project")
)

type Repository interface {
	LockRuntimeCredential(context.Context, *xorm.Session, string) (*model.ProjectAPIKey, error)
	FindRuntimeProjectTx(context.Context, *xorm.Session, int64) (*model.Project, error)
	FindRuntimeConfigTx(context.Context, *xorm.Session, int64, string) (*model.Config, error)
	FindReleasedSnapshotTx(context.Context, *xorm.Session, int64, int64) (*model.Snapshot, error)
	MarkUsed(context.Context, string, time.Time) error
}

type Result struct {
	Content string
	Version uint64
}

type ContentCache struct {
	store cache.Store
}

func NewContentCache(store cache.Store) (*ContentCache, error) {
	if store == nil {
		return nil, fmt.Errorf("runtime content cache store is nil")
	}
	return &ContentCache{store: store}, nil
}

func (c *ContentCache) DeletePublishedConfigVersion(ctx context.Context, environmentID, projectID int64, configKey string, runtimeVersion int64) error {
	key, err := contentCacheKey(environmentID, projectID, configKey, runtimeVersion)
	if err != nil {
		return err
	}
	return c.store.Delete(ctx, key)
}

type Logic struct {
	repository   Repository
	credentials  *credential.Manager
	transactions database.Transactor
	cache        *ContentCache
	now          func() time.Time
}

func New(repository Repository, credentials *credential.Manager, transactions database.Transactor, contentCache *ContentCache) (*Logic, error) {
	if repository == nil || credentials == nil || transactions == nil || contentCache == nil {
		return nil, fmt.Errorf("runtime logic dependencies are incomplete")
	}
	return &Logic{repository: repository, credentials: credentials, transactions: transactions, cache: contentCache, now: time.Now}, nil
}

func (l *Logic) Read(ctx context.Context, fullCredential, requestedProject, configKey string) (*Result, error) {
	publicID, err := l.credentials.PublicID(fullCredential)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	if strings.TrimSpace(requestedProject) == "" || strings.TrimSpace(configKey) == "" {
		return nil, base.InvalidArgument("project and config key are required")
	}

	var result *Result
	err = l.transactions.WithTx(ctx, func(tx *xorm.Session) error {
		key, err := l.repository.LockRuntimeCredential(ctx, tx, publicID)
		if errors.Is(err, base.ErrNotFound) {
			return ErrUnauthenticated
		}
		if err != nil {
			return err
		}
		if key == nil {
			return fmt.Errorf("runtime API key repository returned nil key")
		}
		verified := l.credentials.Verify(fullCredential, key.PublicID, key.SecretHash)
		if !verified || key.RevokedAt != nil {
			return ErrUnauthenticated
		}

		project, err := l.repository.FindRuntimeProjectTx(ctx, tx, key.ProjectID)
		if errors.Is(err, base.ErrNotFound) {
			return ErrUnauthenticated
		}
		if err != nil {
			return err
		}
		if project == nil {
			return fmt.Errorf("runtime project repository returned nil project")
		}
		if project.Key != requestedProject {
			return ErrProjectMismatch
		}

		config, err := l.repository.FindRuntimeConfigTx(ctx, tx, project.ID, configKey)
		if err != nil {
			return err
		}
		if config == nil {
			return fmt.Errorf("runtime config repository returned nil config")
		}
		if config.RuntimeVersion < 0 {
			return fmt.Errorf("runtime config version is invalid")
		}
		cacheKey, err := contentCacheKey(project.EnvironmentID, project.ID, config.Key, config.RuntimeVersion)
		if err != nil {
			return err
		}
		content, err := l.cache.store.Get(ctx, cacheKey)
		if err == nil {
			result = &Result{Content: string(content), Version: uint64(config.RuntimeVersion)}
			return nil
		}
		if !errors.Is(err, cache.ErrNotFound) {
			return fmt.Errorf("read runtime content cache: %w", err)
		}

		snapshot, err := l.repository.FindReleasedSnapshotTx(ctx, tx, project.ID, config.ID)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return fmt.Errorf("runtime snapshot repository returned nil snapshot")
		}
		decoded, err := entity.DecodeConfigSnapshot(snapshot.Content)
		if err != nil {
			return fmt.Errorf("decode released runtime snapshot: %w", err)
		}
		if decoded.Config.GetId() != config.ID || decoded.Config.GetProjectId() != project.ID || decoded.Config.GetKey() != config.Key {
			return fmt.Errorf("released runtime snapshot scope does not match config")
		}
		if err := l.cache.store.Set(ctx, cacheKey, []byte(decoded.Config.GetValue()), contentTTL); err != nil {
			return fmt.Errorf("write runtime content cache: %w", err)
		}
		result = &Result{Content: decoded.Config.GetValue(), Version: uint64(config.RuntimeVersion)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("runtime read returned no result")
	}
	if err := l.repository.MarkUsed(ctx, publicID, l.now().UTC()); err != nil {
		return nil, err
	}
	return result, nil
}

func contentCacheKey(environmentID, projectID int64, configKey string, runtimeVersion int64) (string, error) {
	if environmentID <= 0 || projectID <= 0 || runtimeVersion < 0 || strings.TrimSpace(configKey) == "" {
		return "", base.InvalidArgument("runtime cache scope is invalid")
	}
	return "runtime:config:" + strconv.FormatInt(environmentID, 10) + ":" + strconv.FormatInt(projectID, 10) + ":" + configKey + ":" + strconv.FormatInt(runtimeVersion, 10), nil
}
