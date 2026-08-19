package importer

import (
	"context"
	"fmt"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	exportservice "github.com/yvvlee/kirby/server/internal/service/export"
)

type TransferService struct {
	exporter *exportservice.Service
	importer *Service
}

func NewTransferService(exporter *exportservice.Service, importer *Service) (*TransferService, error) {
	if exporter == nil || importer == nil {
		return nil, fmt.Errorf("snapshot transfer services are incomplete")
	}
	return &TransferService{exporter: exporter, importer: importer}, nil
}

var _ adminv1.SnapshotTransferServiceHTTPServer = (*TransferService)(nil)

func (s *TransferService) ExportSnapshot(ctx context.Context, request *adminv1.ExportSnapshotRequest) (*adminv1.ExportSnapshotReply, error) {
	return s.exporter.ExportSnapshot(ctx, request)
}

func (s *TransferService) ImportSnapshot(ctx context.Context, request *adminv1.ImportSnapshotRequest) (*adminv1.ImportSnapshotReply, error) {
	return s.importer.ImportSnapshot(ctx, request)
}
