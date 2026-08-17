package user

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type fakeUserRepository struct {
	created *model.User
	audit   *model.AuditLog
}

func (*fakeUserRepository) ListManagedUsers(context.Context) ([]model.User, error) { return nil, nil }
func (f *fakeUserRepository) GetByID(_ context.Context, id int64) (*model.User, error) {
	result := *f.created
	result.ID = id
	return &result, nil
}
func (f *fakeUserRepository) CreateManagedUser(_ context.Context, user *model.User, audit *model.AuditLog) error {
	f.created = user
	f.created.ID = 17
	f.audit = audit
	return nil
}
func (*fakeUserRepository) UpdateManagedUser(context.Context, int64, string, bool, int64, int64, *model.AuditLog) (*model.User, error) {
	return nil, nil
}
func (*fakeUserRepository) UpdateManagedPassword(context.Context, int64, string, int64, *model.AuditLog) error {
	return nil
}
func (*fakeUserRepository) UpdateManagedStatus(context.Context, int64, bool, int64, int64, *model.AuditLog) (*model.User, error) {
	return nil, nil
}

type fakeUserAuthorizer struct{ allowed bool }

func (f *fakeUserAuthorizer) RequireSystem(context.Context, int64, string) error {
	if f.allowed {
		return nil
	}
	return permission.ErrForbidden
}

type fakePasswordHasher struct {
	input string
	hash  string
}

func (f *fakePasswordHasher) Hash(value string) (string, error) {
	f.input = value
	return f.hash, nil
}

func TestCreateHashesPasswordAndWritesAudit(t *testing.T) {
	users := new(fakeUserRepository)
	hasher := &fakePasswordHasher{hash: "$argon2id$hash"}
	logic, err := New(users, &fakeUserAuthorizer{allowed: true}, hasher)
	if err != nil {
		t.Fatal(err)
	}
	actor := permission.Actor{UserID: 9, RequestID: "request-4"}
	created, err := logic.Create(context.Background(), actor, "alice", "Alice", "plain", false)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != 17 || hasher.input != "plain" || users.created.PasswordHash != hasher.hash || users.created.PasswordHash == hasher.input {
		t.Fatalf("password was not safely transformed: %#v", users.created)
	}
	if users.audit == nil || users.audit.Action != "user.create" || users.audit.RequestID != actor.RequestID || users.audit.ActorUserID == nil || *users.audit.ActorUserID != actor.UserID {
		t.Fatalf("unexpected audit: %#v", users.audit)
	}
}

func TestOrdinaryUserCannotCreateGlobalUser(t *testing.T) {
	users := new(fakeUserRepository)
	hasher := &fakePasswordHasher{hash: "hash"}
	logic, _ := New(users, &fakeUserAuthorizer{}, hasher)
	_, err := logic.Create(context.Background(), permission.Actor{UserID: 3}, "alice", "Alice", "plain", false)
	if !errors.Is(err, permission.ErrForbidden) || users.created != nil || hasher.input != "" {
		t.Fatalf("unauthorized create error=%v user=%#v hash-input=%q", err, users.created, hasher.input)
	}
}
