package entity

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/permission"
)

// CreateEnumParam 创建枚举入参
type CreateEnumParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ConfigID      int64
	Key           string
	Name          string
	Description   string
	Values        []*commonv1.SelectOption
}

// UpdateEnumParam 更新枚举入参
type UpdateEnumParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	EnumID        int64
	Key           string
	Name          string
	Description   string
	Values        []*commonv1.SelectOption
	Version       int64
}
