package role

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type fakeRoleRepository struct {
	assignments []repository.PermissionAssignment
	audit       *model.AuditLog
	updated     bool
}

func (*fakeRoleRepository) List(context.Context) ([]repository.RoleWithPermissions, error) {
	return nil, nil
}
func (*fakeRoleRepository) FindByID(context.Context, int64) (*repository.RoleWithPermissions, error) {
	return nil, nil
}
func (*fakeRoleRepository) Create(context.Context, *model.Role, *model.AuditLog) error { return nil }
func (*fakeRoleRepository) Update(context.Context, int64, repository.RoleUpdate, *model.AuditLog) error {
	return nil
}
func (*fakeRoleRepository) Delete(context.Context, int64, int64, *model.AuditLog) error { return nil }
func (f *fakeRoleRepository) UpdatePermissions(_ context.Context, _ int64, _ []int64, _ int64, audit *model.AuditLog) ([]repository.PermissionAssignment, error) {
	f.updated = true
	f.audit = audit
	return append([]repository.PermissionAssignment(nil), f.assignments...), nil
}

type fakePermissionRepository struct{}

func (*fakePermissionRepository) List(context.Context) ([]model.Permission, error) { return nil, nil }

type fakeRoleAuthorizer struct {
	allowed     bool
	invalidated [][2]int64
	failures    map[[2]int64]error
}

func (f *fakeRoleAuthorizer) RequireSystem(context.Context, int64, string) error {
	if f.allowed {
		return nil
	}
	return permission.ErrForbidden
}
func (f *fakeRoleAuthorizer) Invalidate(_ context.Context, userID, environmentID int64) error {
	assignment := [2]int64{userID, environmentID}
	f.invalidated = append(f.invalidated, assignment)
	return f.failures[assignment]
}

func TestUpdatePermissionsAttemptsAllInvalidationsAndJoinsFailures(t *testing.T) {
	first := errors.New("first cache unavailable")
	second := errors.New("second cache unavailable")
	roles := &fakeRoleRepository{assignments: []repository.PermissionAssignment{{UserID: 3, EnvironmentID: 10}, {UserID: 4, EnvironmentID: 20}}}
	authorizer := &fakeRoleAuthorizer{allowed: true, failures: map[[2]int64]error{{3, 10}: first, {4, 20}: second}}
	logic, _ := New(roles, new(fakePermissionRepository), authorizer)
	err := logic.UpdatePermissions(context.Background(), permission.Actor{UserID: 9, RequestID: "request-3b"}, 5, []int64{1, 2})
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("joined invalidation error = %v", err)
	}
	if len(authorizer.invalidated) != 2 {
		t.Fatalf("not all invalidations attempted: %#v", authorizer.invalidated)
	}
}

func TestUpdatePermissionsAuditsAndInvalidatesEveryAssignment(t *testing.T) {
	roles := &fakeRoleRepository{assignments: []repository.PermissionAssignment{{UserID: 3, EnvironmentID: 10}, {UserID: 4, EnvironmentID: 20}}}
	authorizer := &fakeRoleAuthorizer{allowed: true}
	logic, err := New(roles, new(fakePermissionRepository), authorizer)
	if err != nil {
		t.Fatal(err)
	}
	actor := permission.Actor{UserID: 9, RequestID: "request-3"}
	if err := logic.UpdatePermissions(context.Background(), actor, 5, []int64{1, 2}); err != nil {
		t.Fatal(err)
	}
	if !roles.updated || roles.audit == nil || roles.audit.Action != "role.permissions.update" || roles.audit.RequestID != actor.RequestID {
		t.Fatalf("unexpected role update audit: %#v", roles.audit)
	}
	want := [][2]int64{{3, 10}, {4, 20}}
	if len(authorizer.invalidated) != len(want) {
		t.Fatalf("invalidations = %#v", authorizer.invalidated)
	}
	for index := range want {
		if authorizer.invalidated[index] != want[index] {
			t.Fatalf("invalidation %d = %#v", index, authorizer.invalidated[index])
		}
	}
}

func TestOrdinaryUserCannotMutateRolePermissions(t *testing.T) {
	roles := new(fakeRoleRepository)
	authorizer := &fakeRoleAuthorizer{allowed: false}
	logic, _ := New(roles, new(fakePermissionRepository), authorizer)
	err := logic.UpdatePermissions(context.Background(), permission.Actor{UserID: 3}, 5, []int64{1})
	if !errors.Is(err, permission.ErrForbidden) || roles.updated {
		t.Fatalf("unauthorized mutation error=%v updated=%v", err, roles.updated)
	}
}
