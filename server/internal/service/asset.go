package service

import (
	"context"
	"errors"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	errorsv1 "github.com/yvvlee/kirby/server/api/errors"
	authsession "github.com/yvvlee/kirby/server/internal/auth/session"
	"github.com/yvvlee/kirby/server/internal/conf"
	"github.com/yvvlee/kirby/server/internal/logic"
)

// AssetService implements adminv1.AssetServiceHTTPServer.
type AssetService struct {
	assetLogic *logic.AssetLogic
	origins    *authsession.OriginValidator
	sameOrigin bool
}

// NewAssetService constructs the HTTP service adapter and public address policy.
// Parameters: cfg supplies storage and proxy settings; logicLayer manages uploads.
// Returns: the asset HTTP service.
// Throws: an error when a configured trusted proxy network is invalid.
func NewAssetService(cfg *conf.Config, logicLayer *logic.AssetLogic) (*AssetService, error) {
	origins, err := authsession.NewOriginValidator(cfg.Security.TrustedProxies)
	if err != nil {
		return nil, err
	}
	return &AssetService{
		assetLogic: logicLayer,
		origins:    origins,
		sameOrigin: cfg.ObjectStorage.Driver == "s3" && cfg.ObjectStorage.S3.PublicBaseURL == "",
	}, nil
}

// PresignAsset authorizes and creates a direct-upload ticket.
// Parameters: ctx carries the caller; request describes the asset and project scope.
// Returns: a signed upload ticket.
// Throws: an API error when authorization, validation, or storage fails.
func (service *AssetService) PresignAsset(ctx context.Context, request *adminv1.PresignAssetRequest) (*adminv1.PresignAssetReply, error) {
	ticket, err := service.assetLogic.Presign(ctx, logic.PresignInput{
		EnvironmentID: request.GetEnvironmentId(),
		ProjectID:     request.GetProjectId(),
		Filename:      request.GetFilename(),
		ContentType:   request.GetContentType(),
		Size:          request.GetSize(),
	})
	if err != nil {
		return nil, assetPublicError(err)
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
// Parameters: ctx carries the HTTP request; request identifies the uploaded object.
// Returns: published metadata with an absolute S3 resource URL for other applications.
// Throws: an API error for an invalid public address, upload, or inaccessible scope.
func (service *AssetService) CompleteAsset(ctx context.Context, request *adminv1.CompleteAssetRequest) (*adminv1.CompleteAssetReply, error) {
	origin, err := service.assetOrigin(ctx)
	if err != nil {
		return nil, err
	}
	metadata, err := service.assetLogic.Complete(ctx, request.GetEnvironmentId(), request.GetProjectId(), request.GetObjectKey())
	if err != nil {
		return nil, assetPublicError(err)
	}
	return &adminv1.CompleteAssetReply{
		Asset: &commonv1.Asset{
			ObjectKey:   metadata.Key,
			Url:         origin + metadata.URL,
			ContentType: metadata.ContentType,
			Size:        metadata.Size,
		},
	}, nil
}

func (service *AssetService) assetOrigin(ctx context.Context) (string, error) {
	if !service.sameOrigin {
		return "", nil
	}
	request, ok := kratoshttp.RequestFromServerContext(ctx)
	if !ok {
		return "", errorsv1.ErrorInternal("HTTP request is unavailable")
	}
	origin, err := service.origins.PublicOrigin(request)
	if err != nil {
		return "", errorsv1.ErrorBadRequest("public request address is invalid")
	}
	return origin, nil
}

func assetPublicError(err error) error {
	switch {
	case errors.Is(err, logic.ErrInvalidInput):
		return errorsv1.ErrorInvalidParam("asset request is invalid")
	case errors.Is(err, logic.ErrForbidden):
		return errorsv1.ErrorForbidden("asset operation is not permitted")
	case errors.Is(err, logic.ErrProjectNotFound), errors.Is(err, logic.ErrAssetNotFound):
		return errorsv1.ErrorNotFound("asset scope was not found")
	case errors.Is(err, logic.ErrAssetConflict):
		return errorsv1.ErrorConflict("asset upload was already completed")
	case errors.Is(err, logic.ErrAssetIntegrity):
		return errorsv1.ErrorBadRequest("uploaded asset failed integrity validation")
	default:
		return errorsv1.ErrorInternal("asset operation failed")
	}
}

var _ adminv1.AssetServiceHTTPServer = (*AssetService)(nil)
