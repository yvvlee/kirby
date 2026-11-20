package service

import (
	"context"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type exportLogic interface {
	Export(ctx context.Context, param *entity.ExportSnapshotParam) (*commonv1.Snapshot, error)
}

type ExportService struct {
	exportLogic exportLogic
}

// NewExportService creates ExportService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *ExportService.
// Throws: none.
func NewExportService(logicLayer *logic.ExportLogic) *ExportService {
	return &ExportService{exportLogic: logicLayer}
}

// ExportSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ExportSnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *ExportService) ExportSnapshot(ctx context.Context, request *adminv1.ExportSnapshotRequest) (*adminv1.ExportSnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.exportLogic.Export(ctx, &entity.ExportSnapshotParam{
		Actor:               actor,
		SourceEnvironmentID: request.SourceEnvironmentId,
		SnapshotID:          request.SnapshotId,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.ExportSnapshotReply{
		SourceEnvironmentId: request.SourceEnvironmentId,
		Snapshot:            item,
	}, nil
}
