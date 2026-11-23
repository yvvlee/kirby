package service

import (
	"context"
	"math"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	"github.com/yvvlee/kirby/server/internal/entity"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type snapshotLogic interface {
	Create(ctx context.Context, param *entity.CreateSnapshotParam) (*commonv1.Snapshot, error)
	Preview(ctx context.Context, actor permission.Actor, environmentID, configID int64) (string, error)
	Delete(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) error
	Get(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64) (*commonv1.Snapshot, error)
	Load(ctx context.Context, param *entity.LoadSnapshotParam) (*commonv1.Snapshot, error)
	Current(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*commonv1.Snapshot, error)
	Released(ctx context.Context, actor permission.Actor, environmentID, configID int64) (*commonv1.Snapshot, error)
	List(ctx context.Context, param *entity.ListSnapshotParam) (repository.PageResult[*commonv1.SimpleSnapshot], error)
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
	item, err := s.snapshotLogic.Create(ctx, &entity.CreateSnapshotParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ProjectID:     request.ProjectId,
		ConfigID:      request.ConfigId,
		Description:   request.Description,
		Tags:          request.Tags,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: item}, nil
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
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: item}, nil
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
	item, err := s.snapshotLogic.Load(ctx, &entity.LoadSnapshotParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ConfigID:      request.ConfigId,
		SnapshotID:    request.Id,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: item}, nil
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
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: item}, nil
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
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: item}, nil
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
	if pageNumber == 0 || limit == 0 || limit > repository.MaxPageSize {
		return nil, errorsv1.ErrorBadRequest("invalid pagination")
	}
	requestedOffset := (uint64(pageNumber) - 1) * uint64(limit)
	if requestedOffset > math.MaxInt {
		return nil, errorsv1.ErrorBadRequest("pagination offset exceeds supported range")
	}
	offset := int(requestedOffset)
	pageLimit := int(limit)
	page, err := s.snapshotLogic.List(ctx, &entity.ListSnapshotParam{
		Actor:         actor,
		EnvironmentID: request.EnvironmentId,
		ProjectID:     request.ProjectId,
		ConfigID:      request.ConfigId,
		Page: repository.PageRequest{
			Offset: offset,
			Limit:  pageLimit,
		},
	})
	if err != nil {
		return nil, apiError(err)
	}
	if page.Limit < 0 || uint64(page.Limit) > math.MaxUint32 || page.Total < 0 {
		return nil, errorsv1.ErrorInternal("invalid pagination result")
	}
	responseLimit := uint32(page.Limit)
	total := uint64(page.Total)
	return &adminv1.ListSnapshotReply{
		Page: &commonv1.Pagination{
			Page:  pageNumber,
			Limit: responseLimit,
			Total: total,
		},
		List: page.Items,
	}, nil
}
