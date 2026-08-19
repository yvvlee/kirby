package exporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
)

type logicFake struct{}

func (logicFake) Export(context.Context, permission.Actor, int64, int64) (*model.Snapshot, error) {
	return nil, nil
}

func TestExportServiceRequiresCurrentJWTIdentity(t *testing.T) {
	_, err := (&Service{logic: logicFake{}}).ExportSnapshot(context.Background(), &adminv1.ExportSnapshotRequest{SourceEnvironmentId: 1, SnapshotId: 12})
	assert.Error(t, err)
}
