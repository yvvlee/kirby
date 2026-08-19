package importer

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/importer"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type Logic interface {
	Import(context.Context, permission.Actor, logic.Request) (*logic.Result, error)
}

type Service struct {
	logic Logic
}

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("snapshot import service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

func (s *Service) ImportSnapshot(ctx context.Context, request *adminv1.ImportSnapshotRequest) (*adminv1.ImportSnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, entity.APIError(entity.Invalid("invalid request"))
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	result, err := s.logic.Import(ctx, actor, logic.Request{
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
		return nil, entity.APIError(err)
	}
	if result == nil || result.Snapshot == nil {
		return nil, entity.APIError(fmt.Errorf("snapshot import returned no result"))
	}
	converted, err := converter.SnapshotToProto(result.Snapshot)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.ImportSnapshotReply{Snapshot: converted, Replayed: result.Replayed}, nil
}
