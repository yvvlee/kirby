// Package permission resolves server-side, environment-scoped authorization.
package permission

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	kratoslog "github.com/go-kratos/kratos/v2/log"

	"github.com/yvvlee/kirby/server/internal/infra/cache"
	"github.com/yvvlee/kirby/server/internal/repository"
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
	SystemProjectManage     = "system:project:manage"

	cacheTTL        = time.Minute
	maxResolveRetry = 3
)

var (
	ErrForbidden           = errors.New("permission denied")
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrConcurrentChange    = errors.New("permissions changed during resolution")
)

type source interface {
	Identity(ctx context.Context, userID, environmentID int64) (repository.PermissionIdentity, error)
	SystemAdmin(ctx context.Context, userID int64) (bool, error)
	KeysForUserEnvironment(ctx context.Context, userID, environmentID int64) ([]string, error)
	List(ctx context.Context) ([]repositoryPermission, error)
}

// repositoryPermission is the subset required by Resolver. A concrete adapter
// below keeps the public repository model out of tests.
type repositoryPermission struct{ Key string }

type repositorySource struct {
	repository.PermissionRepository
}

// List projects the permission catalog to the keys required by authorization.
//
// Parameters: ctx controls the catalog read.
// Returns: permission keys, or the repository error.
// Throws: none; failures are returned as errors.
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

// NewResolver constructs the Resolver with explicit application dependencies.
//
// Parameters: source reads current identities and grants; cacheStore caches generation-specific permission sets.
// Returns: the configured Resolver.
// Throws: none.
func NewResolver(source repository.PermissionRepository, cacheStore cache.Store) *Resolver {
	return &Resolver{
		source: repositorySource{PermissionRepository: source},
		cache:  cacheStore,
	}
}

func newResolver(source source, cacheStore cache.Store) *Resolver {
	return &Resolver{
		source: source,
		cache:  cacheStore,
	}
}

// Resolve rechecks account status and generation before accepting cached permission keys.
//
// Parameters: ctx controls reads; userID selects the account; environmentID defines the scope.
// Returns: normalized permission keys and administrator status, or a denial, dependency error, or ErrConcurrentChange after three attempts.
// Throws: none; failures are returned as errors.
func (r *Resolver) Resolve(ctx context.Context, userID, environmentID int64) ([]string, bool, error) {
	for attempt := 0; attempt < maxResolveRetry; attempt++ {
		keys, systemAdmin, err := r.resolveAttempt(ctx, userID, environmentID)
		if errors.Is(err, ErrConcurrentChange) {
			continue
		}
		return keys, systemAdmin, err
	}
	return nil, false, ErrConcurrentChange
}

func (r *Resolver) resolveAttempt(ctx context.Context, userID, environmentID int64) ([]string, bool, error) {
	identity, err := r.identity(ctx, userID, environmentID)
	if err != nil {
		return nil, false, err
	}
	if identity.SystemAdmin {
		keys, err := r.allPermissionKeys(ctx)
		return keys, true, err
	}
	contentKey := permissionContentKey(userID, environmentID, identity.EnvironmentVersion)
	keys, cached, err := r.loadPermissions(ctx, userID, environmentID, contentKey)
	if err != nil {
		return nil, false, err
	}
	confirmed, err := r.identity(ctx, userID, environmentID)
	if err != nil {
		return nil, false, err
	}
	if confirmed.SystemAdmin {
		keys, err := r.allPermissionKeys(ctx)
		return keys, true, err
	}
	if confirmed.EnvironmentVersion != identity.EnvironmentVersion {
		return nil, false, ErrConcurrentChange
	}
	if !cached {
		r.cachePermissions(ctx, contentKey, keys)
	}
	return keys, false, nil
}

func (r *Resolver) loadPermissions(ctx context.Context, userID, environmentID int64, key string) ([]string, bool, error) {
	if keys, cached := r.cachedPermissions(ctx, key); cached {
		return keys, true, nil
	}
	keys, err := r.source.KeysForUserEnvironment(ctx, userID, environmentID)
	if err != nil {
		return nil, false, err
	}
	return normalize(keys), false, nil
}

func (r *Resolver) cachedPermissions(ctx context.Context, key string) ([]string, bool) {
	encoded, err := r.cache.Get(ctx, key)
	if errors.Is(err, cache.ErrNotFound) {
		return nil, false
	}
	if err != nil {
		kratoslog.Context(ctx).Warnw("operation", "read permission cache", "error", err)
		return nil, false
	}
	keys, err := decodePermissions(encoded)
	if err != nil {
		kratoslog.Context(ctx).Warnw("operation", "decode permission cache", "error", err)
		return nil, false
	}
	return keys, true
}

// Only a database-confirmed permission set reaches this cache write.
func (r *Resolver) cachePermissions(ctx context.Context, key string, keys []string) {
	encoded, err := json.Marshal(keys)
	if err != nil {
		kratoslog.Context(ctx).Errorw("operation", "encode permission cache", "error", err)
		return
	}
	if err := r.cache.Set(ctx, key, encoded, cacheTTL); err != nil {
		kratoslog.Context(ctx).Warnw("operation", "write permission cache", "error", err)
	}
}

func (r *Resolver) identity(ctx context.Context, userID, environmentID int64) (repository.PermissionIdentity, error) {
	identity, err := r.source.Identity(ctx, userID, environmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return repository.PermissionIdentity{}, ErrForbidden
		}
		return repository.PermissionIdentity{}, err
	}
	if identity.EnvironmentID == 0 {
		if identity.SystemAdmin {
			return repository.PermissionIdentity{}, ErrEnvironmentNotFound
		}
		return repository.PermissionIdentity{}, ErrForbidden
	}
	if !identity.SystemAdmin && (!identity.EnvironmentEnabled || !identity.EnvironmentMember) {
		return repository.PermissionIdentity{}, ErrForbidden
	}
	if identity.EnvironmentVersion < 0 {
		return repository.PermissionIdentity{}, fmt.Errorf("invalid environment permission version")
	}
	return identity, nil
}

// Require requires every requested environment permission unless the account is a current system administrator.
//
// Parameters: ctx controls resolution; userID and environmentID define the scope; required lists mandatory permission keys.
// Returns: nil when authorized, ErrForbidden for a missing permission, or a resolution error.
// Throws: none; failures are returned as errors.
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

// RequireSystem checks current enabled-account status for a system-only permission.
//
// Parameters: ctx controls the read; userID selects the account; required must name a system permission.
// Returns: nil for a system administrator, ErrForbidden, an invalid-key error, or a database error.
// Throws: none; failures are returned as errors.
func (r *Resolver) RequireSystem(ctx context.Context, userID int64, required string) error {
	if !strings.HasPrefix(required, "system:") {
		return fmt.Errorf("system permission key required")
	}
	isSystemAdmin, err := r.source.SystemAdmin(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrForbidden
		}
		return err
	}
	if !isSystemAdmin {
		return ErrForbidden
	}
	return nil
}

// Invalidate reclaims the previous permission-cache generation without affecting current authorization.
//
// Parameters: ctx controls cleanup; userID and environmentID identify the stale cache entry.
// Returns: nil after cleanup or when no previous identity exists, or an identity-read or cache-delete error.
// Throws: none; failures are returned as errors.
func (r *Resolver) Invalidate(ctx context.Context, userID, environmentID int64) error {
	identity, err := r.source.Identity(ctx, userID, environmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("read permission identity for invalidation: %w", err)
	}
	previousVersion := identity.EnvironmentVersion - 1
	if previousVersion < 0 {
		return nil
	}
	if err := r.cache.Delete(ctx, permissionContentKey(userID, environmentID, previousVersion)); err != nil {
		return fmt.Errorf("delete stale permission cache: %w", err)
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
