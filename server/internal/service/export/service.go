package exporter

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/export"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type Logic interface {
	Export(context.Context, permission.Actor, int64, int64) (*model.Snapshot, error)
}

type Service struct {
	logic Logic
}

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("snapshot export service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

func (s *Service) ExportSnapshot(ctx context.Context, request *adminv1.ExportSnapshotRequest) (*adminv1.ExportSnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Export(ctx, actor, request.SourceEnvironmentId, request.SnapshotId)
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.SnapshotToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ExportSnapshotReply{SourceEnvironmentId: request.SourceEnvironmentId, Snapshot: converted}, nil
}
