package permission

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/repository/base"
	"github.com/yvvlee/kirby/server/internal/storage/cache"
)

type blockingContentCache struct {
	cache.Store
	mu      sync.Mutex
	started chan struct{}
	release chan struct{}
}

func (c *blockingContentCache) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := c.Store.Get(ctx, key)
	c.mu.Lock()
	started, release := c.started, c.release
	if started != nil && strings.Contains(key, ":version:") {
		c.started = nil
		c.release = nil
	}
	c.mu.Unlock()
	if started != nil && strings.Contains(key, ":version:") {
		started <- struct{}{}
		<-release
	}
	return value, err
}

type fakeSource struct {
	mu          sync.Mutex
	identities  map[[2]int64]repository.PermissionIdentity
	keys        map[[2]int64][]string
	admins      map[int64]bool
	permissions []repositoryPermission
	keyStarted  chan struct{}
	keyRelease  chan struct{}
}

func (f *fakeSource) Identity(_ context.Context, userID, environmentID int64) (repository.PermissionIdentity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	identity, ok := f.identities[[2]int64{userID, environmentID}]
	if !ok {
		return repository.PermissionIdentity{}, base.ErrNotFound
	}
	return identity, nil
}

func (f *fakeSource) SystemAdmin(_ context.Context, userID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.admins[userID]
	if !ok {
		return false, base.ErrNotFound
	}
	return value, nil
}

func (f *fakeSource) KeysForUserEnvironment(_ context.Context, userID, environmentID int64) ([]string, error) {
	f.mu.Lock()
	value := append([]string(nil), f.keys[[2]int64{userID, environmentID}]...)
	started, release := f.keyStarted, f.keyRelease
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		<-release
	}
	return value, nil
}

func (f *fakeSource) List(context.Context) ([]repositoryPermission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]repositoryPermission(nil), f.permissions...), nil
}

func roleMatrixSource() *fakeSource {
	viewer := []string{ProjectRead, ProjectAPIKeyRead, ConfigRead, StructureRead, EnumRead, SnapshotRead, SnapshotExport}
	editor := append(append([]string(nil), viewer...), ProjectWrite, ConfigWrite, StructureWrite, EnumWrite, SnapshotWrite, SnapshotImport, AssetWrite)
	publisher := append(append([]string(nil), editor...), SnapshotPublish)
	admin := append(append([]string(nil), publisher...), ProjectAPIKeyManage, EnvironmentMemberManage)
	return &fakeSource{
		identities: map[[2]int64]repository.PermissionIdentity{
			{1, 10}: {EnvironmentID: 10, EnvironmentEnabled: true, EnvironmentMember: true},
			{2, 10}: {EnvironmentID: 10, EnvironmentEnabled: true, EnvironmentMember: true},
			{3, 10}: {EnvironmentID: 10, EnvironmentEnabled: true, EnvironmentMember: true},
			{4, 10}: {EnvironmentID: 10, EnvironmentEnabled: true, EnvironmentMember: true},
			{9, 10}: {SystemAdmin: true, EnvironmentID: 10, EnvironmentEnabled: true},
		},
		keys:        map[[2]int64][]string{{1, 10}: viewer, {2, 10}: editor, {3, 10}: publisher, {4, 10}: admin},
		admins:      map[int64]bool{1: false, 2: false, 3: false, 4: false, 9: true},
		permissions: []repositoryPermission{{ProjectRead}, {SnapshotPublish}, {SystemUserManage}},
	}
}

func TestBuiltinRolePermissionMatrix(t *testing.T) {
	resolver, err := newResolver(roleMatrixSource(), cache.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		userID  int64
		allowed []string
		denied  string
	}{
		{name: "viewer", userID: 1, allowed: []string{ProjectRead, SnapshotExport}, denied: ConfigWrite},
		{name: "editor", userID: 2, allowed: []string{ConfigWrite, SnapshotImport}, denied: SnapshotPublish},
		{name: "publisher", userID: 3, allowed: []string{SnapshotPublish, ConfigWrite}, denied: EnvironmentMemberManage},
		{name: "admin", userID: 4, allowed: []string{EnvironmentMemberManage, ProjectAPIKeyManage}, denied: SystemUserManage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := resolver.Require(context.Background(), test.userID, 10, test.allowed...); err != nil {
				t.Fatalf("allowed permissions rejected: %v", err)
			}
			if err := resolver.Require(context.Background(), test.userID, 10, test.denied); !errors.Is(err, ErrForbidden) {
				t.Fatalf("denied permission error = %v", err)
			}
		})
	}
}

func TestSystemAdminBypassesEnvironmentRoleButEnvironmentMustExist(t *testing.T) {
	resolver, err := newResolver(roleMatrixSource(), cache.NewMemory())
	if err != nil {
		t.Fatal(err)
	}
	if err := resolver.Require(context.Background(), 9, 10, SystemUserManage, SnapshotPublish); err != nil {
		t.Fatal(err)
	}
	source := roleMatrixSource()
	source.identities[[2]int64{9, 11}] = repository.PermissionIdentity{SystemAdmin: true}
	resolver, _ = newResolver(source, cache.NewMemory())
	if err := resolver.Require(context.Background(), 9, 11, ProjectRead); !errors.Is(err, ErrEnvironmentNotFound) {
		t.Fatalf("missing environment error = %v", err)
	}
}

func TestCrossEnvironmentAccessDoesNotReusePermissionCache(t *testing.T) {
	source := roleMatrixSource()
	source.identities[[2]int64{1, 20}] = repository.PermissionIdentity{EnvironmentID: 20, EnvironmentEnabled: true, EnvironmentMember: true}
	source.keys[[2]int64{1, 20}] = []string{}
	resolver, _ := newResolver(source, cache.NewMemory())
	if err := resolver.Require(context.Background(), 1, 10, ProjectRead); err != nil {
		t.Fatal(err)
	}
	if err := resolver.Require(context.Background(), 1, 20, ProjectRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-environment permission leaked: %v", err)
	}
	if permissionVersionKey(1, 10) == permissionVersionKey(1, 20) {
		t.Fatal("permission cache key omits environment")
	}
}

func TestForeignAndMissingEnvironmentAreIndistinguishableToOrdinaryUser(t *testing.T) {
	source := roleMatrixSource()
	source.identities[[2]int64{1, 20}] = repository.PermissionIdentity{EnvironmentID: 20, EnvironmentEnabled: true}
	source.identities[[2]int64{1, 30}] = repository.PermissionIdentity{}
	resolver, _ := newResolver(source, cache.NewMemory())
	for _, environmentID := range []int64{20, 30} {
		_, _, err := resolver.Resolve(context.Background(), 1, environmentID)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("environment %d disclosed existence: %v", environmentID, err)
		}
	}
}

func TestInvalidationPreventsConcurrentStaleWriter(t *testing.T) {
	source := roleMatrixSource()
	source.keyStarted = make(chan struct{}, 1)
	source.keyRelease = make(chan struct{})
	resolver, _ := newResolver(source, cache.NewMemory())

	result := make(chan []string, 1)
	errorsChannel := make(chan error, 1)
	go func() {
		keys, _, err := resolver.Resolve(context.Background(), 1, 10)
		result <- keys
		errorsChannel <- err
	}()
	<-source.keyStarted
	release := source.keyRelease
	source.mu.Lock()
	source.keys[[2]int64{1, 10}] = []string{ConfigWrite}
	source.keyStarted = nil
	source.keyRelease = nil
	source.mu.Unlock()
	if err := resolver.Invalidate(context.Background(), 1, 10); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-errorsChannel; err != nil {
		t.Fatal(err)
	}
	keys := <-result
	if len(keys) != 1 || keys[0] != ConfigWrite {
		t.Fatalf("stale permissions survived concurrent invalidation: %v", keys)
	}

	cached, _, err := resolver.Resolve(context.Background(), 1, 10)
	if err != nil || len(cached) != 1 || cached[0] != ConfigWrite {
		t.Fatalf("subsequent permissions = %v, err=%v", cached, err)
	}
}

func TestInvalidationPreventsConcurrentCachedStaleReader(t *testing.T) {
	source := roleMatrixSource()
	store := &blockingContentCache{Store: cache.NewMemory()}
	resolver, _ := newResolver(source, store)
	if _, _, err := resolver.Resolve(context.Background(), 1, 10); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.started = make(chan struct{}, 1)
	store.release = make(chan struct{})
	started, release := store.started, store.release
	store.mu.Unlock()

	result := make(chan []string, 1)
	errorsChannel := make(chan error, 1)
	go func() {
		keys, _, err := resolver.Resolve(context.Background(), 1, 10)
		result <- keys
		errorsChannel <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("cached permission read did not start")
	}
	source.mu.Lock()
	source.keys[[2]int64{1, 10}] = []string{ConfigWrite}
	source.mu.Unlock()
	if err := resolver.Invalidate(context.Background(), 1, 10); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-errorsChannel; err != nil {
		t.Fatal(err)
	}
	keys := <-result
	if len(keys) != 1 || keys[0] != ConfigWrite {
		t.Fatalf("stale cached permissions survived invalidation: %v", keys)
	}
}
