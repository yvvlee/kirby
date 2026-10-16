package service

import adminv1 "github.com/yvvlee/kirby/server/api/admin"

type TransferService struct {
	*ExportService
	*ImportService
}

// NewTransferService creates TransferService with its application operations.
//
// Parameters: exporter handles snapshot export requests; importer handles snapshot import requests.
// Returns: *TransferService.
// Throws: none.
func NewTransferService(exporter *ExportService, importer *ImportService) *TransferService {
	return &TransferService{
		ExportService: exporter,
		ImportService: importer,
	}
}

var _ adminv1.SnapshotTransferServiceHTTPServer = (*TransferService)(nil)
