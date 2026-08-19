// Package asset exposes the management HTTP asset contract.
package asset

import (
	"context"
	"errors"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	assetlogic "github.com/yvvlee/kirby/server/internal/logic/asset"
)

// Service implements adminv1.AssetServiceHTTPServer.
type Service struct {
	logic *assetlogic.Logic
}

// New constructs the HTTP service adapter.
func New(logic *assetlogic.Logic) (*Service, error) {
	if logic == nil {
		return nil, fmt.Errorf("asset logic is nil")
	}
	return &Service{logic: logic}, nil
}

// PresignAsset authorizes and creates a direct-upload ticket.
func (service *Service) PresignAsset(ctx context.Context, request *adminv1.PresignAssetRequest) (*adminv1.PresignAssetReply, error) {
	if request == nil {
		return nil, errorsv1.ErrorInvalidParam("request is required")
	}
	if err := request.ValidateAll(); err != nil {
		return nil, errorsv1.ErrorInvalidParam("asset request is invalid")
	}
	ticket, err := service.logic.Presign(ctx, assetlogic.PresignInput{
		EnvironmentID: request.GetEnvironmentId(),
		ProjectID:     request.GetProjectId(),
		Filename:      request.GetFilename(),
		ContentType:   request.GetContentType(),
		Size:          request.GetSize(),
	})
	if err != nil {
		return nil, publicError(err)
	}
	return &adminv1.PresignAssetReply{
		ObjectKey:    ticket.Key,
		UploadUrl:    ticket.URL,
		Headers:      ticket.Headers,
		ExpiresAt:    ticket.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		UploadMethod: ticket.Method,
		FormFields:   ticket.FormFields,
	}, nil
}

// CompleteAsset verifies provider metadata and returns the canonical asset URL.
func (service *Service) CompleteAsset(ctx context.Context, request *adminv1.CompleteAssetRequest) (*adminv1.CompleteAssetReply, error) {
	if request == nil {
		return nil, errorsv1.ErrorInvalidParam("request is required")
	}
	if err := request.ValidateAll(); err != nil {
		return nil, errorsv1.ErrorInvalidParam("asset request is invalid")
	}
	metadata, err := service.logic.Complete(ctx, request.GetEnvironmentId(), request.GetProjectId(), request.GetObjectKey())
	if err != nil {
		return nil, publicError(err)
	}
	return &adminv1.CompleteAssetReply{Asset: &commonv1.Asset{
		ObjectKey:   metadata.Key,
		Url:         metadata.URL,
		ContentType: metadata.ContentType,
		Size:        metadata.Size,
	}}, nil
}

func publicError(err error) error {
	switch {
	case errors.Is(err, assetlogic.ErrInvalidInput):
		return errorsv1.ErrorInvalidParam("asset request is invalid")
	case errors.Is(err, assetlogic.ErrForbidden):
		return errorsv1.ErrorForbidden("asset operation is not permitted")
	case errors.Is(err, assetlogic.ErrProjectNotFound), errors.Is(err, assetlogic.ErrAssetNotFound):
		return errorsv1.ErrorNotFound("asset scope was not found")
	case errors.Is(err, assetlogic.ErrAssetConflict):
		return errorsv1.ErrorConflict("asset upload was already completed")
	case errors.Is(err, assetlogic.ErrAssetIntegrity):
		return errorsv1.ErrorBadRequest("uploaded asset failed integrity validation")
	default:
		return errorsv1.ErrorInternal("asset operation failed")
	}
}

var _ adminv1.AssetServiceHTTPServer = (*Service)(nil)
