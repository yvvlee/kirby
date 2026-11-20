package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
)

// CreateAPIKeyParam 创建项目 API Key 入参
type CreateAPIKeyParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	Name          string
}

// RotateAPIKeyParam 轮换项目 API Key 入参
type RotateAPIKeyParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	KeyID         int64
}

// RevokeAPIKeyParam 吊销项目 API Key 入参
type RevokeAPIKeyParam struct {
	Actor         permission.Actor
	EnvironmentID int64
	ProjectID     int64
	KeyID         int64
}
