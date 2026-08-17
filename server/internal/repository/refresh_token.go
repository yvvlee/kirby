package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"xorm.io/xorm"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/storage/database"
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenExpired  = errors.New("refresh token expired")
	ErrRefreshTokenReplay   = errors.New("refresh token replay detected")
)

// RefreshTokenRepository persists only SHA-256 refresh-token hashes.
type RefreshTokenRepository struct {
	engine *xorm.Engine
}

func NewRefreshTokenRepository(engine *xorm.Engine) (*RefreshTokenRepository, error) {
	if engine == nil {
		return nil, fmt.Errorf("refresh-token repository database is nil")
	}
	return &RefreshTokenRepository{engine: engine}, nil
}

// CreateSession inserts the first refresh token and records login time in one
// transaction. A concurrent user disable causes the whole operation to fail.
func (r *RefreshTokenRepository) CreateSession(ctx context.Context, token *model.RefreshToken, loginAt time.Time) error {
	if err := validateNewRefreshToken(token); err != nil {
		return err
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		result, err := tx.Exec(
			"UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ? AND enabled = TRUE AND deleted_at IS NULL",
			loginAt.UTC(), loginAt.UTC(), token.UserID,
		)
		if err != nil {
			return fmt.Errorf("record user login: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read user login update count: %w", err)
		}
		if affected != 1 {
			return ErrUserNotFound
		}
		if _, err := tx.InsertOne(token); err != nil {
			return fmt.Errorf("create refresh session: %w", err)
		}
		return nil
	})
}

// Rotate consumes the presented token and inserts its replacement atomically.
// Replaying an already rotated token revokes every active token in that session.
func (r *RefreshTokenRepository) Rotate(ctx context.Context, tokenHash []byte, next *model.RefreshToken, now time.Time) (*model.RefreshToken, error) {
	if len(tokenHash) != 32 {
		return nil, ErrRefreshTokenNotFound
	}
	if next == nil || len(next.TokenHash) != 32 || !next.ExpiresAt.After(now) {
		return nil, fmt.Errorf("invalid replacement refresh token")
	}

	var rotated *model.RefreshToken
	var outcome error
	err := database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		current := new(model.RefreshToken)
		found, err := tx.Where("token_hash = ?", tokenHash).ForUpdate().Get(current)
		if err != nil {
			return fmt.Errorf("lock refresh token: %w", err)
		}
		if !found {
			outcome = ErrRefreshTokenNotFound
			return nil
		}
		if current.RevokedAt != nil {
			if current.ReplacedByID != nil {
				if _, err := tx.Exec(
					"UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, ?), updated_at = ? WHERE user_id = ? AND session_id = ?",
					now.UTC(), now.UTC(), current.UserID, current.SessionID,
				); err != nil {
					return fmt.Errorf("revoke replayed refresh session: %w", err)
				}
				outcome = ErrRefreshTokenReplay
				return nil
			}
			outcome = ErrRefreshTokenNotFound
			return nil
		}
		if !now.Before(current.ExpiresAt) {
			outcome = ErrRefreshTokenExpired
			return nil
		}

		next.UserID = current.UserID
		next.SessionID = current.SessionID
		if _, err := tx.InsertOne(next); err != nil {
			return fmt.Errorf("insert replacement refresh token: %w", err)
		}
		result, err := tx.Exec(
			"UPDATE refresh_tokens SET last_used_at = ?, revoked_at = ?, replaced_by_id = ?, updated_at = ? "+
				"WHERE id = ? AND revoked_at IS NULL",
			now.UTC(), now.UTC(), next.ID, now.UTC(), current.ID,
		)
		if err != nil {
			return fmt.Errorf("consume refresh token: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read consumed refresh count: %w", err)
		}
		if affected != 1 {
			return fmt.Errorf("refresh token changed while locked")
		}
		rotated = next
		return nil
	})
	if err != nil {
		return nil, err
	}
	if outcome != nil {
		return nil, outcome
	}
	return rotated, nil
}

// RevokeSessionByTokenHash makes logout idempotent. Unknown hashes do not
// reveal whether a browser supplied a real session token.
func (r *RefreshTokenRepository) RevokeSessionByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) error {
	if len(tokenHash) != 32 {
		return nil
	}
	return database.WithTx(ctx, r.engine, func(tx *xorm.Session) error {
		current := new(model.RefreshToken)
		found, err := tx.Where("token_hash = ?", tokenHash).ForUpdate().Get(current)
		if err != nil {
			return fmt.Errorf("lock refresh session for revocation: %w", err)
		}
		if !found {
			return nil
		}
		if _, err := tx.Exec(
			"UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, ?), updated_at = ? WHERE user_id = ? AND session_id = ?",
			now.UTC(), now.UTC(), current.UserID, current.SessionID,
		); err != nil {
			return fmt.Errorf("revoke refresh session: %w", err)
		}
		return nil
	})
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
