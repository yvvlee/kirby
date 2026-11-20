package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

// CreateProjectParam 创建项目入参
type CreateProjectParam struct {
	Actor       permission.Actor
	Key         string
	Name        string
	Description string
}

// UpdateProjectParam 更新项目入参
type UpdateProjectParam struct {
	Actor     permission.Actor
	ProjectID int64
	Update    repository.ProjectUpdate
}

// ListProjectParam 项目列表查询入参
type ListProjectParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	Keyword       string
}
