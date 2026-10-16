package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type setupTestRepository struct {
	repository.SetupRepository
	initialized bool
	statusErr   error
	createErr   error
	created     *model.User
	createCtx   context.Context
}

func (r *setupTestRepository) IsInitialized(context.Context) (bool, error) {
	return r.initialized, r.statusErr
}

func (r *setupTestRepository) CreateInitialAdmin(ctx context.Context, user *model.User) error {
	r.created = user
	r.createCtx = ctx
	return r.createErr
}

type setupTestHasher struct {
	err error
}

func (h setupTestHasher) Hash(string) (string, error) {
	return "", h.err
}

func setupTestPasswordHasher(t *testing.T) *password.Hasher {
	t.Helper()
	hasher, err := password.New(password.Params{
		MemoryKiB:   8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	if err != nil {
		t.Fatal(err)
	}
	return hasher
}

func TestSetupStatusPropagatesDatabaseFailure(t *testing.T) {
	failure := errors.New("database unavailable")
	repo := &setupTestRepository{statusErr: failure}
	setup := NewSetupLogic(repo, setupTestPasswordHasher(t), directTransactions{})
	if _, err := setup.GetSetupStatus(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("status error = %v", err)
	}
}

func TestSetupCreatesAdminWithHashedPasswordInsideTransaction(t *testing.T) {
	repo := &setupTestRepository{}
	hasher := setupTestPasswordHasher(t)
	type transactionKey struct{}
	txctx := context.WithValue(context.Background(), transactionKey{}, true)
	called := false
	store := transactionFunc(func(_ context.Context, fn func(context.Context) error) error {
		called = true
		return fn(txctx)
	})
	setup := NewSetupLogic(repo, hasher, store)
	if err := setup.CreateInitialAdmin(context.Background(), "owner", "Owner", "initial-password"); err != nil {
		t.Fatal(err)
	}
	if !called || repo.createCtx != txctx {
		t.Fatal("repository was called outside the setup transaction")
	}
	user := repo.created
	if user.Username != "owner" || user.DisplayName != "Owner" || !user.Enabled || !user.IsSystemAdmin || user.MustChangePassword {
		t.Fatalf("unexpected initial administrator: %+v", user)
	}
	match, _, err := hasher.Verify(user.PasswordHash, "initial-password")
	if err != nil || !match {
		t.Fatalf("initial password was not hashed: match=%v err=%v", match, err)
	}
}

func TestSetupStopsBeforeTransactionWhenHashFails(t *testing.T) {
	failure := errors.New("random source unavailable")
	repo := &setupTestRepository{}
	called := false
	store := transactionFunc(func(context.Context, func(context.Context) error) error {
		called = true
		return nil
	})
	setup := NewSetupLogic(repo, setupTestPasswordHasher(t), store)
	setup.passwords = setupTestHasher{err: failure}
	if err := setup.CreateInitialAdmin(context.Background(), "owner", "Owner", "initial-password"); !errors.Is(err, failure) {
		t.Fatalf("hash error = %v", err)
	}
	if called || repo.created != nil {
		t.Fatal("setup touched persistence after hashing failed")
	}
}

func TestSetupPropagatesTransactionAndAlreadyInitializedErrors(t *testing.T) {
	failure := errors.New("transaction commit failed")
	repo := &setupTestRepository{}
	setup := NewSetupLogic(repo, setupTestPasswordHasher(t), transactionFunc(func(context.Context, func(context.Context) error) error { return failure }))
	if err := setup.CreateInitialAdmin(context.Background(), "owner", "Owner", "initial-password"); !errors.Is(err, failure) {
		t.Fatalf("transaction error = %v", err)
	}
	if repo.created != nil {
		t.Fatal("repository called after transaction start failed")
	}
	repo.createErr = repository.ErrAlreadyInitialized
	setup.transactions = directTransactions{}
	if err := setup.CreateInitialAdmin(context.Background(), "owner", "Owner", "initial-password"); !errors.Is(err, repository.ErrAlreadyInitialized) {
		t.Fatalf("closed setup error = %v", err)
	}
}
