package entity

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// CreateConfigParam 创建配置入参
type CreateConfigParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	Key           string
	Description   string
}

// UpdateConfigParam 更新配置入参
type UpdateConfigParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ConfigID      int64
	Description   string
	FieldType     *commonv1.Field_Type
	IsArray       bool
	Version       int64
}

// UpdateConfigValueParam 更新配置值入参
type UpdateConfigValueParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ConfigID      int64
	Value         string
	Version       int64
}

// ListConfigParam 配置列表查询入参
type ListConfigParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	Filter        repository.ConfigFilter
}
