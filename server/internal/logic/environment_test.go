package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/entity"
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
func (f *fakeEnvironmentRepository) Create(_ context.Context, item *model.Environment, audit *model.AuditLog) (rowsAffected int64, err error) {
	item.ID = 99
	f.items = append(f.items, *item)
	if audit.Action != "environment.create" || audit.RequestID == "" {
		err = errors.New("invalid create audit")
		return
	}
	return
}
func (*fakeEnvironmentRepository) Update(context.Context, int64, repository.EnvironmentUpdate, *model.AuditLog) (rowsAffected int64, err error) {
	return
}

type fakeEnvironmentUsers struct {
	repository.UserRepository
	assigned []model.Environment
}

func (*fakeEnvironmentUsers) GetByID(_ context.Context, id int64) (*model.User, error) {
	return &model.User{
		Meta:    model.Meta{ID: id},
		Enabled: true,
	}, nil
}
func (f *fakeEnvironmentUsers) ListEnvironments(context.Context, *model.User) ([]model.Environment, error) {
	return append([]model.Environment(nil), f.assigned...), nil
}

func environmentTestStorage() TransactionRunner {
	return directTransactions{}
}

type fakeEnvironmentAuthorizer struct {
	system        bool
	allowed       map[int64]bool
	invalidated   [][2]int64
	invalidateErr error
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
	return f.invalidateErr
}

func TestListUsesMembershipForOrdinaryUsersAndSystemAdmins(t *testing.T) {
	all := []model.Environment{{Meta: model.Meta{ID: 1}}, {Meta: model.Meta{ID: 2}}}
	assigned := []model.Environment{{Meta: model.Meta{ID: 2}}}
	environments := &fakeEnvironmentRepository{items: all}
	users := &fakeEnvironmentUsers{assigned: assigned}
	authorizer := &fakeEnvironmentAuthorizer{allowed: map[int64]bool{}}
	logic := NewEnvironmentLogic(environments, users, authorizer, environmentTestStorage(), memberRejectRepo{actor: "OWNER"})

	items, err := logic.List(context.Background(), permission.Actor{UserID: 7})
	if err != nil || len(items) != 1 || items[0].Id != 2 {
		t.Fatalf("ordinary list = %#v, err=%v", items, err)
	}
	authorizer.system = true
	items, err = logic.List(context.Background(), permission.Actor{UserID: 7})
	if err != nil || len(items) != 1 || items[0].Id != 2 {
		t.Fatalf("system list = %#v, err=%v", items, err)
	}
}

func TestEnvironmentManagementRequiresProjectOwner(t *testing.T) {
	for _, role := range []string{"MAINTAINER", "MEMBER", "GUEST", "UNKNOWN"} {
		t.Run(role, func(t *testing.T) {
			l := &EnvironmentLogic{
				projectMembers: memberRejectRepo{
					actor: role,
				},
			}
			_, err := l.Create(context.Background(), &entity.CreateEnvironmentParam{
				Actor:     permission.Actor{UserID: 7},
				ProjectID: 1,
			})
			if !errors.Is(err, permission.ErrForbidden) {
				t.Fatalf("create allowed for %s: %v", role, err)
			}
		})
	}
	l := &EnvironmentLogic{
		projectMembers: memberRejectRepo{actor: "OWNER"},
	}
	if err := l.requireProjectOwner(context.Background(), 7, 2); err == nil {
		t.Fatal("cross-project owner accepted")
	}
}
