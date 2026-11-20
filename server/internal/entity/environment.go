package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// CreateEnvironmentParam 创建环境入参
type CreateEnvironmentParam struct {
	Actor       permission.Actor
	ProjectID   int64
	Key         string
	Name        string
	Description string
}

// UpdateEnvironmentParam 更新环境入参
type UpdateEnvironmentParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	Update        repository.EnvironmentUpdate
}

// UpdateMemberRolesParam 更新环境成员角色入参
