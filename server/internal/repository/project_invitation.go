package repository

import (
	"context"
	"time"

	"github.com/yvvlee/lorm"
	"github.com/yvvlee/lorm/builder"

	"github.com/yvvlee/kirby/server/internal/infra"
	"github.com/yvvlee/kirby/server/internal/model"
)

type ProjectInvitationRepository interface {
	Create(ctx context.Context, item *model.ProjectInvitation) error
	GetActive(ctx context.Context, tokenHash []byte, now time.Time) (*model.ProjectInvitation, error)
	Accept(ctx context.Context, id, userID int64, at time.Time) error
	Revoke(ctx context.Context, id int64) error
}

type ProjectInvitationRepositoryImpl struct {
	*lorm.Repository[*model.ProjectInvitation]
}

// NewProjectInvitationRepository uses the shared database to persist single-use invitation state.
// Parameters: dependencies provide persistence and authorization.
// Returns: the configured component.
// Throws: none; errors are returned.
func NewProjectInvitationRepository(storage *infra.Storage) *ProjectInvitationRepositoryImpl {
	return &ProjectInvitationRepositoryImpl{Repository: storage.CoreDB.Repository[*model.ProjectInvitation]()}
}

// Create stores only the invitation digest so database access does not reveal usable links.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ProjectInvitationRepositoryImpl) Create(ctx context.Context, item *model.ProjectInvitation) error {
	if _, err := r.Insert(ctx, item); err != nil {
		return WrapDatabaseError(err)
	}
	return nil
}

// GetActive rejects consumed, expired and revoked invitation links at lookup.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ProjectInvitationRepositoryImpl) GetActive(ctx context.Context, tokenHash []byte, now time.Time) (*model.ProjectInvitation, error) {
	f := new(model.ProjectInvitation).LormCols()
	item, found, err := r.Engine.Query[*model.ProjectInvitation]().Where(builder.And{
		builder.Eq{f.TokenHash(): tokenHash},
		builder.Gt(f.ExpiresAt(), now),
		builder.IsNull(f.AcceptedAt()), builder.IsNull(f.RevokedAt()),
	}).Get(ctx)
	if err != nil {
		return nil, WrapDatabaseError(err)
	}
	if !found {
		return nil, Missing("project invitation")
	}
	return item, nil
}

// Accept consumes a still-active invitation with a conditional update to prevent reuse.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ProjectInvitationRepositoryImpl) Accept(ctx context.Context, id, userID int64, at time.Time) error {
	f := new(model.ProjectInvitation).LormCols()
	rows, err := r.Engine.Update[*model.ProjectInvitation]().Where(builder.And{builder.Eq{f.ID(): id}, builder.IsNull(f.AcceptedAt()), builder.IsNull(f.RevokedAt()), builder.Gt(f.ExpiresAt(), at)}).SetMap(map[string]any{
		f.AcceptedBy(): userID,
		f.AcceptedAt(): at,
	}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	if rows == 0 {
		return Missing("active project invitation")
	}
	return nil
}

// Revoke invalidates an invitation without exposing its secret.
// Parameters: ctx carries cancellation and identity; remaining inputs identify the requested operation.
// Returns: the operation result or an error when authorization or persistence fails.
// Throws: none; errors are returned.
func (r *ProjectInvitationRepositoryImpl) Revoke(ctx context.Context, id int64) error {
	f := new(model.ProjectInvitation).LormCols()
	_, err := r.Engine.Update[*model.ProjectInvitation]().ID(id).SetMap(map[string]any{f.RevokedAt(): time.Now().UTC()}).Exec(ctx)
	if err != nil {
		return WrapDatabaseError(err)
	}
	return nil
}

var _ ProjectInvitationRepository = (*ProjectInvitationRepositoryImpl)(nil)
