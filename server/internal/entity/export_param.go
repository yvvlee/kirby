package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
)

// ExportSnapshotParam 导出快照入参
type ExportSnapshotParam struct {
	Actor               permission.Actor
	SourceEnvironmentID int64
	SnapshotID          int64
}
