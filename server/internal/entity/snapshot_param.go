package entity

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// CreateSnapshotParam 创建快照入参
type CreateSnapshotParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	ConfigID      int64
	Description   string
	Tags          []commonv1.Snapshot_Tag
}

// ListSnapshotParam 快照列表查询入参
type ListSnapshotParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	ConfigID      int64
	Page          repository.PageRequest
}

// LoadSnapshotParam 载入快照入参
type LoadSnapshotParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ConfigID      int64
	SnapshotID    int64
}
