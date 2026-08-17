package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type fakeEnvironmentRepository struct {
	items []model.Environment
}

func (f *fakeEnvironmentRepository) List(context.Context) ([]model.Environment, error) {
	return append([]model.Environment(nil), f.items...), nil
}
func (f *fakeEnvironmentRepository) FindByID(_ context.Context, id int64) (*model.Environment, error) {
	for index := range f.items {
		if f.items[index].ID == id {
			item := f.items[index]
			return &item, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeEnvironmentRepository) Create(_ context.Context, item *model.Environment, audit *model.AuditLog) error {
	item.ID = 99
	f.items = append(f.items, *item)
	if audit.Action != "environment.create" || audit.RequestID == "" {
		return errors.New("invalid create audit")
	}
	return nil
}
func (*fakeEnvironmentRepository) Update(context.Context, int64, repository.EnvironmentUpdate, *model.AuditLog) error {
	return nil
}

type fakeEnvironmentUsers struct{ assigned []model.Environment }

func (*fakeEnvironmentUsers) GetByID(_ context.Context, id int64) (*model.User, error) {
	return &model.User{Meta: model.Meta{ID: id}, Enabled: true}, nil
}
func (f *fakeEnvironmentUsers) ListEnvironments(context.Context, *model.User) ([]model.Environment, error) {
	return append([]model.Environment(nil), f.assigned...), nil
}

type fakeMembers struct {
	replaceCalls int
	environment  int64
	user         int64
	roles        []int64
	audit        *model.AuditLog
}

func (*fakeMembers) ListMembers(context.Context, int64) ([]repository.EnvironmentMemberRecord, error) {
	return nil, nil
}
func (f *fakeMembers) ReplaceRoles(_ context.Context, environmentID, userID int64, roles []int64, _ int64, audit *model.AuditLog) error {
	f.replaceCalls++
	f.environment, f.user = environmentID, userID
	f.roles = append([]int64(nil), roles...)
	f.audit = audit
	return nil
}

type fakeEnvironmentAuthorizer struct {
	system      bool
	allowed     map[int64]bool
	invalidated [][2]int64
}

func (*fakeEnvironmentAuthorizer) Resolve(context.Context, int64, int64) ([]string, bool, error) {
	return []string{permission.ProjectRead}, false, nil
}
func (f *fakeEnvironmentAuthorizer) Require(_ context.Context, _, environmentID int64, _ ...string) error {
	if f.allowed[environmentID] {
		return nil
	}
	return permission.ErrForbidden
}
func (f *fakeEnvironmentAuthorizer) RequireSystem(context.Context, int64, string) error {
	if f.system {
		return nil
	}
	return permission.ErrForbidden
}
func (f *fakeEnvironmentAuthorizer) Invalidate(_ context.Context, userID, environmentID int64) error {
	f.invalidated = append(f.invalidated, [2]int64{userID, environmentID})
	return nil
}

func TestUpdateMemberRolesIsEnvironmentScopedAuditedAndInvalidated(t *testing.T) {
	members := new(fakeMembers)
	authorizer := &fakeEnvironmentAuthorizer{allowed: map[int64]bool{10: true}}
	logic, err := New(&fakeEnvironmentRepository{}, &fakeEnvironmentUsers{}, members, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	actor := permission.Actor{UserID: 7, RequestID: "request-1"}
	if err := logic.UpdateMemberRoles(context.Background(), actor, 10, 22, []int64{4}); err != nil {
		t.Fatal(err)
	}
	if members.replaceCalls != 1 || members.environment != 10 || members.user != 22 || len(members.roles) != 1 || members.roles[0] != 4 {
		t.Fatalf("unexpected member mutation: %#v", members)
	}
	if members.audit == nil || members.audit.Action != "environment.member.roles.update" || members.audit.RequestID != actor.RequestID || members.audit.ActorUserID == nil || *members.audit.ActorUserID != actor.UserID {
		t.Fatalf("unexpected audit: %#v", members.audit)
	}
	if len(authorizer.invalidated) != 1 || authorizer.invalidated[0] != [2]int64{22, 10} {
		t.Fatalf("unexpected invalidation: %#v", authorizer.invalidated)
	}
}

func TestUpdateMemberRolesRejectsForeignEnvironmentBeforeRepository(t *testing.T) {
	members := new(fakeMembers)
	authorizer := &fakeEnvironmentAuthorizer{allowed: map[int64]bool{10: true}}
	logic, _ := New(&fakeEnvironmentRepository{}, &fakeEnvironmentUsers{}, members, authorizer)
	err := logic.UpdateMemberRoles(context.Background(), permission.Actor{UserID: 7, RequestID: "request-2"}, 20, 22, []int64{4})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("foreign environment error = %v", err)
	}
	if members.replaceCalls != 0 || len(authorizer.invalidated) != 0 {
		t.Fatal("foreign environment reached mutation or invalidation")
	}
}

func TestListUsesAssignedEnvironmentsForOrdinaryUserAndAllForSystemAdmin(t *testing.T) {
	all := []model.Environment{{Meta: model.Meta{ID: 1}}, {Meta: model.Meta{ID: 2}}}
	assigned := []model.Environment{{Meta: model.Meta{ID: 2}}}
	environments := &fakeEnvironmentRepository{items: all}
	users := &fakeEnvironmentUsers{assigned: assigned}
	authorizer := &fakeEnvironmentAuthorizer{allowed: map[int64]bool{}}
	logic, _ := New(environments, users, new(fakeMembers), authorizer)

	items, err := logic.List(context.Background(), permission.Actor{UserID: 7})
	if err != nil || len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("ordinary list = %#v, err=%v", items, err)
	}
	authorizer.system = true
	items, err = logic.List(context.Background(), permission.Actor{UserID: 7})
	if err != nil || len(items) != 2 {
		t.Fatalf("system list = %#v, err=%v", items, err)
	}
}
