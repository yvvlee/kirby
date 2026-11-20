package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/entity"
)

type exportLogicFake struct {
	param  *entity.ExportSnapshotParam
	result *commonv1.Snapshot
}

func (f *exportLogicFake) Export(_ context.Context, param *entity.ExportSnapshotParam) (*commonv1.Snapshot, error) {
	f.param = param
	return f.result, nil
}

func TestExportServiceRequiresCurrentJWTIdentity(t *testing.T) {
	_, err := (&ExportService{exportLogic: &exportLogicFake{}}).ExportSnapshot(context.Background(), &adminv1.ExportSnapshotRequest{
		SourceEnvironmentId: 1,
		SnapshotId:          12,
	})
	assert.Error(t, err)
}
