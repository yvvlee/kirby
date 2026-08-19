package publish

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/publish"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type Logic interface {
	Publish(context.Context, permission.Actor, int64, int64, uint32) (*model.Snapshot, error)
	Unpublish(context.Context, permission.Actor, int64, int64, uint32) (*model.Snapshot, error)
}

type Service struct {
	logic Logic
}

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("publication service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.PublicationServiceHTTPServer = (*Service)(nil)

func (s *Service) PublishSnapshot(ctx context.Context, request *adminv1.PublishSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Publish(ctx, actor, request.EnvironmentId, request.SnapshotId, request.Version)
	return reply(item, err)
}

func (s *Service) UnpublishSnapshot(ctx context.Context, request *adminv1.PublishSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Unpublish(ctx, actor, request.EnvironmentId, request.SnapshotId, request.Version)
	return reply(item, err)
}

func reply(item *model.Snapshot, err error) (*adminv1.SnapshotReply, error) {
	if err != nil {
		return nil, entity.APIError(err)
	}
	converted, err := converter.SnapshotToProto(item)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: converted}, nil
}

func badRequest() error {
	return entity.APIError(entity.Invalid("invalid request"))
}
