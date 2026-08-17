// Package permission resolves server-side, environment-scoped authorization.
package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

const (
	ProjectRead             = "project:read"
	ProjectWrite            = "project:write"
	ProjectAPIKeyRead       = "project:api_key:read"
	ProjectAPIKeyManage     = "project:api_key:manage"
	ConfigRead              = "config:read"
	ConfigWrite             = "config:write"
	StructureRead           = "structure:read"
	StructureWrite          = "structure:write"
	EnumRead                = "enum:read"
	EnumWrite               = "enum:write"
	SnapshotRead            = "snapshot:read"
	SnapshotWrite           = "snapshot:write"
	SnapshotPublish         = "snapshot:publish"
	SnapshotExport          = "snapshot:export"
	SnapshotImport          = "snapshot:import"
	AssetWrite              = "asset:write"
	EnvironmentMemberManage = "environment:member:manage"
	SystemUserManage        = "system:user:manage"
	SystemRoleManage        = "system:role:manage"
	SystemEnvironmentManage = "system:environment:manage"

	cacheTTL        = time.Minute
	versionTTL      = 24 * time.Hour
	maxResolveRetry = 3
)

var (
	ErrForbidden           = errors.New("permission denied")
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrConcurrentChange    = errors.New("permissions changed during resolution")
)

type source interface {
	Identity(context.Context, int64, int64) (repository.PermissionIdentity, error)
	SystemAdmin(context.Context, int64) (bool, error)
	KeysForUserEnvironment(context.Context, int64, int64) ([]string, error)
	List(context.Context) ([]repositoryPermission, error)
}

// repositoryPermission is the subset required by Resolver. A concrete adapter
// below keeps the public repository model out of tests.
type repositoryPermission struct{ Key string }

type repositorySource struct {
	repository.PermissionRepository
}

func (s repositorySource) List(ctx context.Context) ([]repositoryPermission, error) {
	items, err := s.PermissionRepository.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]repositoryPermission, 0, len(items))
	for _, item := range items {
		result = append(result, repositoryPermission{Key: item.Key})
	}
	return result, nil
}

// Resolver always rechecks current user and environment status in MySQL. Only
// ordinary environment permission sets are cached.
type Resolver struct {
	source source
	cache  cache.Store
}

func NewResolver(source repository.PermissionRepository, cacheStore cache.Store) (*Resolver, error) {
	if source == nil || cacheStore == nil {
		return nil, fmt.Errorf("permission dependencies are incomplete")
	}
	return &Resolver{source: repositorySource{PermissionRepository: source}, cache: cacheStore}, nil
}

func newResolver(source source, cacheStore cache.Store) (*Resolver, error) {
	if source == nil || cacheStore == nil {
		return nil, fmt.Errorf("permission dependencies are incomplete")
	}
	return &Resolver{source: source, cache: cacheStore}, nil
}

// Resolve returns current permissions for one user and one environment.
func (r *Resolver) Resolve(ctx context.Context, userID, environmentID int64) ([]string, bool, error) {
	identity, err := r.source.Identity(ctx, userID, environmentID)
	if err != nil {
		if errors.Is(err, base.ErrNotFound) {
			return nil, false, ErrForbidden
		}
		return nil, false, err
	}
	if identity.EnvironmentID == 0 {
		if identity.SystemAdmin {
			return nil, true, ErrEnvironmentNotFound
		}
		return nil, false, ErrForbidden
	}
	if identity.SystemAdmin {
		keys, err := r.allPermissionKeys(ctx)
		return keys, true, err
	}
	if !identity.EnvironmentEnabled || !identity.EnvironmentMember {
		return nil, false, ErrForbidden
	}

	for attempt := 0; attempt < maxResolveRetry; attempt++ {
		version, err := r.version(ctx, userID, environmentID)
		if err != nil {
			return nil, false, err
		}
		contentKey := permissionContentKey(userID, environmentID, version)
		encoded, err := r.cache.Get(ctx, contentKey)
		if err == nil {
			keys, err := decodePermissions(encoded)
			if err != nil {
				return nil, false, err
			}
			confirmed, err := r.currentVersion(ctx, userID, environmentID)
			if err != nil {
				return nil, false, err
			}
			if confirmed != version {
				continue
			}
			return keys, false, nil
		}
		if !errors.Is(err, cache.ErrNotFound) {
			return nil, false, fmt.Errorf("read permission cache: %w", err)
		}

		keys, err := r.source.KeysForUserEnvironment(ctx, userID, environmentID)
		if err != nil {
			return nil, false, err
		}
		keys = normalize(keys)
		confirmed, err := r.currentVersion(ctx, userID, environmentID)
		if err != nil {
			return nil, false, err
		}
		if confirmed != version {
			continue
		}
		encoded, err = json.Marshal(keys)
		if err != nil {
			return nil, false, fmt.Errorf("encode permissions: %w", err)
		}
		if err := r.cache.Set(ctx, contentKey, encoded, cacheTTL); err != nil {
			return nil, false, fmt.Errorf("write permission cache: %w", err)
		}
		return keys, false, nil
	}
	return nil, false, ErrConcurrentChange
}

func (r *Resolver) Require(ctx context.Context, userID, environmentID int64, required ...string) error {
	keys, systemAdmin, err := r.Resolve(ctx, userID, environmentID)
	if err != nil {
		return err
	}
	if systemAdmin {
		return nil
	}
	available := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		available[key] = struct{}{}
	}
	for _, key := range required {
		if _, ok := available[key]; !ok {
			return ErrForbidden
		}
	}
	return nil
}

func (r *Resolver) RequireSystem(ctx context.Context, userID int64, required string) error {
	if !strings.HasPrefix(required, "system:") {
		return fmt.Errorf("system permission key required")
	}
	isSystemAdmin, err := r.source.SystemAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, base.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if !isSystemAdmin {
		return ErrForbidden
	}
	return nil
}

// Invalidate increments a shared generation before deleting the old content
// key. A concurrent stale writer can only populate an obsolete generation.
func (r *Resolver) Invalidate(ctx context.Context, userID, environmentID int64) error {
	oldVersion, err := r.currentVersion(ctx, userID, environmentID)
	if err != nil && !errors.Is(err, cache.ErrNotFound) {
		return fmt.Errorf("read permission version for invalidation: %w", err)
	}
	newVersion, err := r.cache.Increment(ctx, permissionVersionKey(userID, environmentID), versionTTL)
	if err != nil {
		return fmt.Errorf("increment permission version: %w", err)
	}
	if oldVersion > 0 && newVersion != oldVersion {
		if err := r.cache.Delete(ctx, permissionContentKey(userID, environmentID, oldVersion)); err != nil {
			return fmt.Errorf("delete stale permission cache: %w", err)
		}
	}
	return nil
}

func (r *Resolver) allPermissionKeys(ctx context.Context) ([]string, error) {
	permissions, err := r.source.List(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(permissions))
	for _, item := range permissions {
		keys = append(keys, item.Key)
	}
	return normalize(keys), nil
}

func (r *Resolver) version(ctx context.Context, userID, environmentID int64) (int64, error) {
	version, err := r.currentVersion(ctx, userID, environmentID)
	if err == nil {
		return version, nil
	}
	if !errors.Is(err, cache.ErrNotFound) {
		return 0, err
	}
	version, err = r.cache.Increment(ctx, permissionVersionKey(userID, environmentID), versionTTL)
	if err != nil {
		return 0, fmt.Errorf("initialize permission version: %w", err)
	}
	return version, nil
}

func (r *Resolver) currentVersion(ctx context.Context, userID, environmentID int64) (int64, error) {
	value, err := r.cache.Get(ctx, permissionVersionKey(userID, environmentID))
	if err != nil {
		return 0, err
	}
	version, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("invalid permission cache version")
	}
	return version, nil
}

func permissionVersionKey(userID, environmentID int64) string {
	return fmt.Sprintf("permission:user:%d:environment:%d:version", userID, environmentID)
}

func permissionContentKey(userID, environmentID, version int64) string {
	return fmt.Sprintf("permission:user:%d:environment:%d:version:%d", userID, environmentID, version)
}

func decodePermissions(value []byte) ([]string, error) {
	var keys []string
	if err := json.Unmarshal(value, &keys); err != nil {
		return nil, fmt.Errorf("decode permission cache: %w", err)
	}
	return normalize(keys), nil
}

func normalize(keys []string) []string {
	result := append([]string(nil), keys...)
	sort.Strings(result)
	write := 0
	for _, key := range result {
		if key == "" || (write > 0 && result[write-1] == key) {
			continue
		}
		result[write] = key
		write++
	}
	return result[:write]
}
