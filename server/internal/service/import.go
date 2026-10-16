package service

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type importLogic interface {
	Import(ctx context.Context, actor permission.Actor, request logic.ImportRequest) (*logic.ImportResult, error)
}

type ImportService struct {
	importLogic importLogic
}

// NewImportService creates ImportService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *ImportService.
// Throws: none.
func NewImportService(logicLayer *logic.ImportLogic) *ImportService {
	return &ImportService{importLogic: logicLayer}
}

// ImportSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ImportSnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ImportService) ImportSnapshot(ctx context.Context, request *adminv1.ImportSnapshotRequest) (*adminv1.ImportSnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	result, err := s.importLogic.Import(ctx, actor, logic.ImportRequest{
		SourceEnvironmentID: request.SourceEnvironmentId,
		SourceSnapshotID:    request.SourceSnapshotId,
		TargetEnvironmentID: request.TargetEnvironmentId,
		TargetProjectID:     request.TargetProjectId,
		TargetConfigID:      request.TargetConfigId,
		Description:         request.Description,
		Tags:                request.Tags,
		IdempotencyKey:      request.IdempotencyKey,
		ConflictStrategy:    logic.ConflictStrategy(request.ConflictStrategy),
	})
	if err != nil {
		return nil, apiError(err)
	}
	if result == nil || result.Snapshot == nil {
		return nil, apiError(fmt.Errorf("snapshot import returned no result"))
	}
	converted, err := converter.SnapshotToProto(result.Snapshot)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ImportSnapshotReply{
		Snapshot: converted,
		Replayed: result.Replayed,
	}, nil
}
