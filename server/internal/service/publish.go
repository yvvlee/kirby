package service

import (
	"context"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/converter"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type publishLogic interface {
	Publish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error)
	Unpublish(ctx context.Context, actor permission.Actor, environmentID, snapshotID int64, expectedVersion uint32) (*model.Snapshot, error)
}

type PublishService struct {
	publishLogic publishLogic
}

// NewPublishService creates PublishService with its application operations.
//
// Parameters: logicLayer supplies the application operations.
// Returns: *PublishService.
// Throws: none.
func NewPublishService(logicLayer *logic.PublishLogic) *PublishService {
	return &PublishService{publishLogic: logicLayer}
}

var _ adminv1.PublicationServiceHTTPServer = (*PublishService)(nil)

// PublishSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *PublishService) PublishSnapshot(ctx context.Context, request *adminv1.PublishSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.publishLogic.Publish(ctx, actor, request.EnvironmentId, request.SnapshotId, request.Version)
	return publicationReply(item, err)
}

// UnpublishSnapshot maps the authenticated API request to the application operation.
//
// Parameters: ctx carries cancellation and transaction state; request contains the validated request fields.
// Returns: *adminv1.SnapshotReply on success, or an API error translated from authentication, validation, or application failure.
// Throws: none.
func (s *PublishService) UnpublishSnapshot(ctx context.Context, request *adminv1.PublishSnapshotRequest) (*adminv1.SnapshotReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	item, err := s.publishLogic.Unpublish(ctx, actor, request.EnvironmentId, request.SnapshotId, request.Version)
	return publicationReply(item, err)
}

func publicationReply(item *model.Snapshot, err error) (*adminv1.SnapshotReply, error) {
	if err != nil {
		return nil, apiError(err)
	}
	converted, err := converter.SnapshotToProto(item)
	if err != nil {
		return nil, apiError(err)
	}
	return &adminv1.SnapshotReply{Snapshot: converted}, nil
}
