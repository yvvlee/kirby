package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
)

// PublishSnapshotParam 发布快照入参
type PublishSnapshotParam struct {
	Actor           permission.Actor
	EnvironmentID   int64
	SnapshotID      int64
	ExpectedVersion uint32
}

// UnpublishSnapshotParam 取消发布快照入参
type UnpublishSnapshotParam struct {
	Actor           permission.Actor
	EnvironmentID   int64
	SnapshotID      int64
	ExpectedVersion uint32
}
