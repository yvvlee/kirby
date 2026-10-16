package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	credential "github.com/yvvlee/kirby/server/internal/auth/api_key"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
)

const contentTTL = 24 * time.Hour

var (
	ErrUnauthenticated = errors.New("runtime API key authentication failed")
	ErrProjectMismatch = errors.New("runtime API key does not belong to requested project")
)

type RuntimeResult struct {
	Content string
	Version uint64
}

type ContentCache struct {
	store cache.Store
}

// NewContentCache creates ContentCache with its configuration dependencies.
//
// Parameters: store provides the shared cache client.
// Returns: *ContentCache.
// Throws: none.
func NewContentCache(store cache.Store) *ContentCache {
	return &ContentCache{store: store}
}

// DeletePublishedConfigVersion removes the stale cache generation after a committed write.
//
// Parameters: ctx carries cancellation and transaction state; projectID selects the project; configKey selects the configuration key; runtimeVersion selects the immutable runtime cache generation.
// Returns: the cache deletion error, or nil when the generation was removed.
// Throws: none.
func (c *ContentCache) DeletePublishedConfigVersion(ctx context.Context, projectID int64, configKey string, runtimeVersion int64) error {
	return c.store.Delete(ctx, contentCacheKey(projectID, configKey, runtimeVersion))
}

type RuntimeLogic struct {
	repository   repository.ProjectAPIKeyRepository
	credentials  *credential.Manager
	transactions TransactionRunner
	cache        *ContentCache
	now          func() time.Time
}

// NewRuntimeLogic creates RuntimeLogic with its configuration dependencies.
//
// Parameters: repository loads runtime API keys and published content; credentials verifies runtime API credentials; transactions commits state changes and audit events together; contentCache stores immutable published-content generations.
// Returns: *RuntimeLogic.
// Throws: none.
func NewRuntimeLogic(
	repository repository.ProjectAPIKeyRepository,
	credentials *credential.Manager,
	transactions TransactionRunner,
	contentCache *ContentCache,
) *RuntimeLogic {
	return &RuntimeLogic{
		repository:   repository,
		credentials:  credentials,
		transactions: transactions,
		cache:        contentCache,
		now:          time.Now,
	}
}

type runtimeRead struct {
	publicID       string
	fullCredential string
	projectKey     string
	configKey      string
}

// Read authenticates the API key before reading the published configuration.
//
// Parameters: ctx carries cancellation and transaction state; fullCredential contains the presented API credential; requestedProject is the project key requested by the client; configKey selects the configuration key.
// Returns: the published value and runtime generation, or a credential, project-scope, cache, decoding, or repository error.
// Throws: none.
func (l *RuntimeLogic) Read(ctx context.Context, fullCredential, requestedProject, configKey string) (*RuntimeResult, error) {
	publicID, err := l.credentials.PublicID(fullCredential)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	request := runtimeRead{
		publicID:       publicID,
		fullCredential: fullCredential,
		projectKey:     requestedProject,
		configKey:      configKey,
	}
	var result *RuntimeResult
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		project, err := l.authenticate(txctx, request)
		if err != nil {
			return err
		}
		result, err = l.readConfig(txctx, project.ID, request.configKey)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := l.repository.MarkUsed(ctx, publicID, l.now().UTC()); err != nil {
		return nil, err
	}
	return result, nil
}

func (l *RuntimeLogic) authenticate(ctx context.Context, request runtimeRead) (*model.Project, error) {
	key, err := l.repository.LockRuntimeCredential(ctx, request.publicID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if !l.credentials.Verify(request.fullCredential, key.PublicID, key.SecretHash) || key.RevokedAt != nil {
		return nil, ErrUnauthenticated
	}
	project, err := l.repository.FindRuntimeProject(ctx, key.ProjectID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if project.Key != request.projectKey {
		return nil, ErrProjectMismatch
	}
	return project, nil
}

func (l *RuntimeLogic) readConfig(ctx context.Context, projectID int64, configKey string) (*RuntimeResult, error) {
	config, err := l.repository.FindRuntimeConfig(ctx, projectID, configKey)
	if err != nil {
		return nil, err
	}
	version, err := safeint.Uint64FromInt64(config.RuntimeVersion)
	if err != nil {
		return nil, fmt.Errorf("runtime config version is invalid: %w", err)
	}
	key := contentCacheKey(projectID, config.Key, config.RuntimeVersion)
	content, err := l.cache.store.Get(ctx, key)
	if err == nil {
		return &RuntimeResult{
			Content: string(content),
			Version: version,
		}, nil
	}
	if !errors.Is(err, cache.ErrNotFound) {
		return nil, fmt.Errorf("read runtime content cache: %w", err)
	}
	value, err := l.releasedContent(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := l.cache.store.Set(ctx, key, []byte(value), contentTTL); err != nil {
		return nil, fmt.Errorf("write runtime content cache: %w", err)
	}
	return &RuntimeResult{
		Content: value,
		Version: version,
	}, nil
}

func (l *RuntimeLogic) releasedContent(ctx context.Context, config *model.Config) (string, error) {
	snapshot, err := l.repository.FindReleasedSnapshot(ctx, config.ProjectID, config.ID)
	if err != nil {
		return "", err
	}
	decoded, err := entity.DecodeConfigSnapshot(snapshot.Content)
	if err != nil {
		return "", fmt.Errorf("decode released runtime snapshot: %w", err)
	}
	if decoded.Config.GetId() != config.ID || decoded.Config.GetProjectId() != config.ProjectID || decoded.Config.GetKey() != config.Key {
		return "", fmt.Errorf("released runtime snapshot scope does not match config")
	}
	return decoded.Config.GetValue(), nil
}

func contentCacheKey(projectID int64, configKey string, runtimeVersion int64) string {
	return "runtime:config:" + strconv.FormatInt(projectID, 10) + ":" + configKey + ":" + strconv.FormatInt(runtimeVersion, 10)
}
