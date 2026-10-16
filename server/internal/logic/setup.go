package logic

import (
	"context"
	"fmt"

	"github.com/yvvlee/kirby/server/internal/auth/password"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// SetupLogic creates the first administrator through the shared database lock.
type SetupLogic struct {
	repo         repository.SetupRepository
	passwords    userPasswordHasher
	transactions TransactionRunner
}

// NewSetupLogic wires first-run setup without process-local initialization state.
// Parameters: repo persists setup state; hasher hashes passwords; transactions runs atomic database operations.
// Returns: the initial administrator setup logic.
// Throws: none.
func NewSetupLogic(repo repository.SetupRepository, hasher *password.Hasher, transactions TransactionRunner) *SetupLogic {
	return &SetupLogic{
		repo:         repo,
		passwords:    hasher,
		transactions: transactions,
	}
}

// GetSetupStatus reads persistent state so every instance sees the same result.
// Parameters: ctx controls the database read.
// Returns: whether administrator setup has completed.
// Throws: the repository error when persistent state cannot be read.
func (l *SetupLogic) GetSetupStatus(ctx context.Context) (bool, error) {
	return l.repo.IsInitialized(ctx)
}

// CreateInitialAdmin hashes credentials before acquiring the database setup lock.
// Parameters: ctx controls the transaction; username, displayName, and plainPassword are validated inputs.
// Returns: nil when the first administrator and permanent setup marker are committed.
// Throws: password hashing, transaction, repository, or already initialized errors.
func (l *SetupLogic) CreateInitialAdmin(ctx context.Context, username, displayName, plainPassword string) error {
	hash, err := l.passwords.Hash(plainPassword)
	if err != nil {
		return fmt.Errorf("hash initial administrator password: %w", err)
	}
	user := &model.User{
		Username:      username,
		DisplayName:   displayName,
		PasswordHash:  hash,
		Enabled:       true,
		IsSystemAdmin: true,
	}
	return l.transactions.TX(ctx, func(txctx context.Context) error {
		return l.repo.CreateInitialAdmin(txctx, user)
	})
}
