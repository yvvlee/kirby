package snapshot

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/gen/kirby/admin/v1"
	commonv1 "github.com/yvvlee/kirby/server/gen/kirby/common/v1"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	logic "github.com/yvvlee/kirby/server/internal/logic/snapshot"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository/base"
)

type Logic interface {
	Create(context.Context, permission.Actor, int64, int64, int64, string, []commonv1.Snapshot_Tag) (*model.Snapshot, error)
	Preview(context.Context, permission.Actor, int64, int64) (string, error)
	Delete(context.Context, permission.Actor, int64, int64) error
	Get(context.Context, permission.Actor, int64, int64) (*model.Snapshot, error)
	Load(context.Context, permission.Actor, int64, int64, int64) (*model.Snapshot, error)
	Current(context.Context, permission.Actor, int64, int64) (*model.Snapshot, error)
	Released(context.Context, permission.Actor, int64, int64) (*model.Snapshot, error)
	List(context.Context, permission.Actor, int64, int64, int64, base.PageRequest) (base.PageResult[model.Snapshot], error)
}
type Service struct{ logic Logic }

func New(logicLayer *logic.Logic) (*Service, error) {
	if logicLayer == nil {
		return nil, fmt.Errorf("snapshot service logic is nil")
	}
	return &Service{logic: logicLayer}, nil
}

var _ adminv1.SnapshotServiceHTTPServer = (*Service)(nil)

func (s *Service) CreateSnapshot(ctx context.Context, request *adminv1.CreateSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Create(ctx, actor, request.EnvironmentId, request.ProjectId, request.ConfigId, request.Description, request.Tags)
	return reply(item, err)
}

func (s *Service) PreviewCreatingSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.PreviewCreatingSnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	content, err := s.logic.Preview(ctx, actor, request.EnvironmentId, request.ConfigId)
	if err != nil {
		return nil, entity.APIError(err)
	}
	return &adminv1.PreviewCreatingSnapshotReply{Content: content}, nil
}

func (s *Service) DeleteSnapshot(ctx context.Context, request *adminv1.SnapshotIDRequest) (*emptypb.Empty, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	if err := s.logic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, entity.APIError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) GetSnapshot(ctx context.Context, request *adminv1.SnapshotIDRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Get(ctx, actor, request.EnvironmentId, request.Id)
	return reply(item, err)
}

func (s *Service) LoadSnapshot(ctx context.Context, request *adminv1.LoadSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Load(ctx, actor, request.EnvironmentId, request.ConfigId, request.Id)
	return reply(item, err)
}

func (s *Service) CurrentSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Current(ctx, actor, request.EnvironmentId, request.ConfigId)
	return reply(item, err)
}

func (s *Service) ReleasedSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.SnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	item, err := s.logic.Released(ctx, actor, request.EnvironmentId, request.ConfigId)
	return reply(item, err)
}

func (s *Service) ListSnapshot(ctx context.Context, request *adminv1.ListSnapshotRequest) (*adminv1.ListSnapshotReply, error) {
	if request == nil || request.ValidateAll() != nil {
		return nil, badRequest()
	}
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, entity.APIError(err)
	}
	pageNumber, limit := uint32(1), uint32(20)
	if request.Page != nil {
		if request.Page.Page != nil {
			pageNumber = *request.Page.Page
		}
		if request.Page.Limit != nil {
			limit = *request.Page.Limit
		}
	}
	page, err := s.logic.List(ctx, actor, request.EnvironmentId, request.ProjectId, request.ConfigId, base.PageRequest{Offset: int((uint64(pageNumber) - 1) * uint64(limit)), Limit: int(limit)})
	if err != nil {
		return nil, entity.APIError(err)
	}
	items := make([]*commonv1.SimpleSnapshot, 0, len(page.Items))
	for index := range page.Items {
		converted, err := converter.SimpleSnapshotToProto(&page.Items[index])
		if err != nil {
			return nil, entity.APIError(err)
		}
		items = append(items, converted)
	}
	return &adminv1.ListSnapshotReply{Page: &commonv1.Pagination{Page: pageNumber, Limit: uint32(page.Limit), Total: uint64(page.Total)}, List: items}, nil
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

func badRequest() error { return entity.APIError(entity.Invalid("invalid request")) }
