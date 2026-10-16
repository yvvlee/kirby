package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenReplay   = errors.New("refresh token replay detected")
)

type RefreshTokenRepository interface {
	CreateSession(ctx context.Context, token *model.RefreshToken, loginAt time.Time) error
	Rotate(ctx context.Context, tokenHash []byte, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error)
	RevokeSessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) error
}

type RefreshTokenRepositoryImpl struct {
	*lorm.Repository[*model.RefreshToken]
}

// NewRefreshTokenRepository binds the RefreshTokenRepository to the application database.
//
// Parameters: storage supplies the shared CoreDB engine and its transaction context.
// Returns: the configured RefreshTokenRepository.
// Throws: none.
func NewRefreshTokenRepository(storage *infra.Storage) *RefreshTokenRepositoryImpl {
	return &RefreshTokenRepositoryImpl{
		Repository: storage.CoreDB.Repository[*model.RefreshToken](),
	}
}

// CreateSession updates the enabled account login time before storing the token digest.
//
// Parameters: ctx carries the login transaction; token is the new session record; loginAt is the login time.
// Returns: nil after both writes, ErrUserNotFound, an invalid-token error, or a database error.
// Throws: none; failures are returned as errors.
func (r *RefreshTokenRepositoryImpl) CreateSession(ctx context.Context, token *model.RefreshToken, loginAt time.Time) error {
	if err := validateNewRefreshToken(token); err != nil {
		return err
	}
	fields := new(model.User).LormCols()
	affected, err := r.Engine.Update[*model.User]().ID(token.UserID).
		Where(builder.And{
			builder.Eq{
				fields.Enabled(): true,
			},
			builder.IsNull(fields.DeletedAt()),
		}).SetMap(map[string]any{
		fields.LastLoginAt(): loginAt.UTC(),
		fields.UpdatedAt():   loginAt.UTC(),
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected != 1 {
		return ErrUserNotFound
	}
	token.PrepareInsert()
	_, err = r.Insert(ctx, token)
	return WrapDatabaseError(err)
}

// Rotate consumes a refresh token once and revokes its session when a consumed token is replayed.
//
// Parameters: ctx carries the rotation transaction; tokenHash identifies the current token; next supplies its replacement digest and expiry; now is the decision time.
// Returns: the replacement token, ErrRefreshTokenNotFound, ErrRefreshTokenExpired, ErrRefreshTokenReplay, or a write error.
// Throws: none; failures are returned as errors.
func (r *RefreshTokenRepositoryImpl) Rotate(ctx context.Context, tokenHash []byte, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	if len(tokenHash) != 32 {
		return nil, ErrRefreshTokenNotFound
	}
	if next == nil || len(next.TokenHash) != 32 || !next.ExpiresAt.After(now) {
		return nil, fmt.Errorf("invalid replacement refresh token")
	}
	current, err := lockTokenByHash(ctx, r.Engine, tokenHash)
	if err != nil {
		return nil, err
	}
	if err := r.checkRotation(ctx, current, now); err != nil {
		return nil, err
	}
	next.UserID = current.UserID
	next.SessionID = current.SessionID
	next.PrepareInsert()
	if _, err := r.Insert(ctx, next); err != nil {
		return nil, WrapDatabaseError(err)
	}
	if err := r.consumeToken(ctx, current.ID, next.ID, now); err != nil {
		return nil, err
	}
	return next, nil
}

func (r *RefreshTokenRepositoryImpl) checkRotation(ctx context.Context, current *model.RefreshToken, now time.Time) error {
	if current == nil {
		return ErrRefreshTokenNotFound
	}
	if current.RevokedAt == nil && !now.Before(current.ExpiresAt) {
		return ErrRefreshTokenExpired
	}
	if current.RevokedAt == nil {
		return nil
	}
	if current.ReplacedByID == nil {
		return ErrRefreshTokenNotFound
	}
	if err := revokeSession(ctx, r.Engine, current.UserID, current.SessionID, now); err != nil {
		return err
	}
	return ErrRefreshTokenReplay
}

func (r *RefreshTokenRepositoryImpl) consumeToken(ctx context.Context, currentID, nextID int64, now time.Time) error {
	fields := new(model.RefreshToken).LormCols()
	affected, err := r.Engine.Update[*model.RefreshToken]().ID(currentID).
		Where(builder.IsNull(fields.RevokedAt())).SetMap(map[string]any{
		fields.LastUsedAt():   now.UTC(),
		fields.RevokedAt():    now.UTC(),
		fields.ReplacedByID(): nextID,
		fields.UpdatedAt():    now.UTC(),
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if affected != 1 {
		return fmt.Errorf("refresh token changed while locked")
	}
	return nil
}

// RevokeSessionByTokenHash makes logout idempotent by revoking all tokens of a matching session.
//
// Parameters: ctx carries the revocation transaction; tokenHash identifies any token in the session; now is the revocation time.
// Returns: nil for revoked or unknown sessions, or a database error.
// Throws: none; failures are returned as errors.
func (r *RefreshTokenRepositoryImpl) RevokeSessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) error {
	if len(tokenHash) != 32 {
		return nil
	}
	current, err := lockTokenByHash(ctx, r.Engine, tokenHash)
	if err != nil {
		return err
	}
	if current == nil {
		return nil
	}
	return revokeSession(ctx, r.Engine, current.UserID, current.SessionID, now)
}

func lockTokenByHash(ctx context.Context, engine *lorm.Engine, tokenHash []byte) (*model.RefreshToken, error) {
	current, _, err := engine.Query[*model.RefreshToken]().
		Where(builder.Eq{new(model.RefreshToken).LormCols().TokenHash(): tokenHash}).
		Suffix("FOR UPDATE").Get(ctx)
	return current, WrapDatabaseError(err)
}

func revokeSession(ctx context.Context, engine *lorm.Engine, userID int64, sessionID string, now time.Time) error {
	fields := new(model.RefreshToken).LormCols()
	_, err := engine.Update[*model.RefreshToken]().Where(builder.And{
		builder.Eq{
			fields.UserID():    userID,
			fields.SessionID(): sessionID,
		},
		builder.IsNull(fields.RevokedAt()),
	}).SetMap(map[string]any{
		fields.RevokedAt(): now.UTC(),
		fields.UpdatedAt(): now.UTC(),
	}).Exec(ctx)
	return WrapDatabaseError(err)
}

func validateNewRefreshToken(token *model.RefreshToken) error {
	if token == nil || token.UserID <= 0 || token.SessionID == "" || len(token.TokenHash) != 32 || token.ExpiresAt.IsZero() {
		return fmt.Errorf("invalid refresh token record")
	}
	if token.RevokedAt != nil || token.ReplacedByID != nil {
		return fmt.Errorf("new refresh token is already revoked")
	}
	return nil
}

var _ RefreshTokenRepository = (*RefreshTokenRepositoryImpl)(nil)
