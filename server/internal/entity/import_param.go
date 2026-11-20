package entity

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/permission"
)

// ConflictStrategy 导入冲突处理策略
type ConflictStrategy int

const (
	// StrategyFail 遇到目标键冲突时直接失败
	StrategyFail ConflictStrategy = 1
	// StrategyReplace 替换指定的目标配置
	StrategyReplace ConflictStrategy = 2
)

// ImportSnapshotParam 导入快照入参
type ImportSnapshotParam struct {
	Actor               permission.Actor
	SourceEnvironmentID int64
	SourceSnapshotID    int64
	TargetEnvironmentID int64
	TargetProjectID     int64
	TargetConfigID      *int64
	Description         string
	Tags                []commonv1.Snapshot_Tag
	IdempotencyKey      string
	ConflictStrategy    ConflictStrategy
}
