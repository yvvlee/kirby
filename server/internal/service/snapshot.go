package service

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
	"github.com/yvvlee/kirby/server/internal/util/safeint"
)

type snapshotLogic interface {
	Create(ctx context.Context, actor permission.Actor, environmentID int64, input logic.SnapshotCreateInput) (*model.Snapshot, error)
	Preview(ctx context.Context, actor permission.Actor, environmentID, configID int64) (string, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) error
	Get(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) (*model.Snapshot, error)
	Load(ctx context.Context, actor permission.Actor, environmentID, configID, snapshotID int64) (*model.Snapshot, error)
	Current(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error)
	Released(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*model.Snapshot, error)
	List(ctx context.Context, actor permission.Actor, environmentID int64, input logic.SnapshotListInput) (repository.PageResult[model.Snapshot], error)
}
type SnapshotService struct{ snapshotLogic snapshotLogic }

// NewSnapshotService creates SnapshotService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *SnapshotService.
// Throws: none.
func NewSnapshotService(logicLayer *logic.SnapshotLogic) *SnapshotService {
	return &SnapshotService{snapshotLogic: logicLayer}
}

var _ adminv1.SnapshotServiceHTTPServer = (*SnapshotService)(nil)

// CreateSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) CreateSnapshot(ctx context.Context, request *adminv1.CreateSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.snapshotLogic.Create(ctx, actor, request.EnvironmentId, logic.SnapshotCreateInput{
		ProjectID:   request.ProjectId,
		ConfigID:    request.ConfigId,
		Description: request.Description,
		Tags:        request.Tags,
	})
	return snapshotReply(item, err)
}

// PreviewCreatingSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.PreviewCreatingSnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) PreviewCreatingSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.PreviewCreatingSnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	content, err := s.snapshotLogic.Preview(ctx, actor, request.EnvironmentId, request.ConfigId)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.PreviewCreatingSnapshotReply{Content: content}, nil
}

// DeleteSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *emptypb.Empty on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) DeleteSnapshot(ctx context.Context, request *adminv1.SnapshotIDRequest) (*emptypb.Empty, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if err := s.snapshotLogic.Delete(ctx, actor, request.EnvironmentId, request.Id); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}

// GetSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) GetSnapshot(ctx context.Context, request *adminv1.SnapshotIDRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.snapshotLogic.Get(ctx, actor, request.EnvironmentId, request.Id)
	return snapshotReply(item, err)
}

// LoadSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) LoadSnapshot(ctx context.Context, request *adminv1.LoadSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.snapshotLogic.Load(ctx, actor, request.EnvironmentId, request.ConfigId, request.Id)
	return snapshotReply(item, err)
}

// CurrentSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) CurrentSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.snapshotLogic.Current(ctx, actor, request.EnvironmentId, request.ConfigId)
	return snapshotReply(item, err)
}

// ReleasedSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) ReleasedSnapshot(ctx context.Context, request *adminv1.ConfigSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.snapshotLogic.Released(ctx, actor, request.EnvironmentId, request.ConfigId)
	return snapshotReply(item, err)
}

// ListSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.ListSnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *SnapshotService) ListSnapshot(ctx context.Context, request *adminv1.ListSnapshotRequest) (*adminv1.ListSnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
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
	offsetValue := (uint64(pageNumber) - 1) * uint64(limit)
	offset, err := safeint.IntFromUint64(offsetValue)
	if err != nil {
		return nil, apiError(entity.Invalid("invalid page offset"))
	}
	pageLimit, err := safeint.IntFromUint32(limit)
	if err != nil {
		return nil, apiError(entity.Invalid("invalid page limit"))
	}
	page, err := s.snapshotLogic.List(ctx, actor, request.EnvironmentId, logic.SnapshotListInput{
		ProjectID: request.ProjectId,
		ConfigID:  request.ConfigId,
		Page: repository.PageRequest{
			Offset: offset,
			Limit:  pageLimit,
		},
	})
	if err != nil {
		return nil, apiError(err)
	}
	items := make([]*commonv1.SimpleSnapshot, 0, len(page.Items))
	for index := range page.Items {
		converted, err := converter.SimpleSnapshotToProto(&page.Items[index])
		if err != nil {
			return nil, apiError(err)
		}
		items = append(items, converted)
	}
	responseLimit, err := safeint.Uint32FromInt(page.Limit)
	if err != nil {
		return nil, apiError(fmt.Errorf("invalid snapshot page limit"))
	}
	total, err := safeint.Uint64FromInt64(page.Total)
	if err != nil {
		return nil, apiError(fmt.Errorf("invalid snapshot page total"))
	}
	return &adminv1.ListSnapshotReply{
		Page: &commonv1.Pagination{
			Page:  pageNumber,
			Limit: responseLimit,
			Total: total,
		},
		List: items,
	}, nil
}

func snapshotReply(item *model.Snapshot, err error) (*adminv1.SnapshotReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	converted, err := converter.SnapshotToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: converted}, nil
}
