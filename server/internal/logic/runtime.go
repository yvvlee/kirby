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
)

const contentTTL = 24 * time.Hour

var ErrUnauthenticated = errors.New("runtime API key authentication failed")

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
// Parameters: ctx carries cancellation and transaction state; configID selects the environment-owned configuration; configKey selects its key; runtimeVersion selects the immutable runtime cache generation.
// Returns: the cache deletion error, or nil when the generation was removed.
// Throws: none.
func (c *ContentCache) DeletePublishedConfigVersion(ctx context.Context, configID int64, configKey string, runtimeVersion int64) error {
	return c.store.Delete(ctx, contentCacheKey(configID, configKey, runtimeVersion))
}

type RuntimeLogic struct {
	repository   repository.ProjectAPIKeyRepository
	credentials  *credential.Manager
	transactions TransactionRunner
	cache        *ContentCache
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
	}
}

type runtimeRead struct {
	secretHash     []byte
	fullCredential string
	configKey      string
}

// Read authenticates the API key before reading the published configuration.
//
// Parameters: ctx carries cancellation and transaction state; fullCredential contains the presented API credential; configKey selects the configuration key.
// Returns: the published value and runtime generation, or a credential, cache, decoding, or repository error.
// Throws: none.
func (l *RuntimeLogic) Read(ctx context.Context, fullCredential, configKey string) (*RuntimeResult, error) {
	secretHash, err := l.credentials.LookupHash(fullCredential)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	request := runtimeRead{
		secretHash:     secretHash,
		fullCredential: fullCredential,
		configKey:      configKey,
	}
	var result *RuntimeResult
	err = l.transactions.TX(ctx, func(txctx context.Context) error {
		key, err := l.authenticate(txctx, request)
		if err != nil {
			return err
		}
		result, err = l.readConfig(txctx, key.EnvironmentID, key.ProjectID, request.configKey)
		return err
	})
	if err != nil {
		return nil, err
	}
	if _, err := l.repository.MarkUsed(ctx, secretHash, time.Now().UTC()); err != nil {
		return nil, err
	}
	return result, nil
}

func (l *RuntimeLogic) authenticate(ctx context.Context, request runtimeRead) (*model.ProjectAPIKey, error) {
	key, err := l.repository.LockRuntimeCredential(ctx, request.secretHash)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if !l.credentials.Verify(request.fullCredential, key.SecretHash) || key.RevokedAt != nil {
		return nil, ErrUnauthenticated
	}
	_, err = l.repository.FindRuntimeProject(ctx, key.EnvironmentID, key.ProjectID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (l *RuntimeLogic) readConfig(ctx context.Context, environmentID, projectID int64, configKey string) (*RuntimeResult, error) {
	config, err := l.repository.FindRuntimeConfig(ctx, environmentID, projectID, configKey)
	if err != nil {
		return nil, err
	}
	version := uint64(config.RuntimeVersion)
	key := contentCacheKey(config.ID, config.Key, config.RuntimeVersion)
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

func contentCacheKey(configID int64, configKey string, runtimeVersion int64) string {
	return "runtime:scoped-config:" + strconv.FormatInt(configID, 10) + ":" + configKey + ":" + strconv.FormatInt(runtimeVersion, 10)
}
