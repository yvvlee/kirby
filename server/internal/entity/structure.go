package entity

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/permission"
)

// CreateStructureParam 创建结构入参
type CreateStructureParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ConfigID      int64
	Key           string
	Name          string
	Description   string
}

// UpdateStructureParam 更新结构入参
type UpdateStructureParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	StructureID   int64
	Key           string
	Name          string
	Description   string
	Fields        []*commonv1.Field
	Version       int64
}

// ListStructureParam 结构列表查询入参
type ListStructureParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	ConfigID      int64
	IgnoreID      *int64
}
