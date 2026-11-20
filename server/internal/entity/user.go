package entity

import (
	"github.com/yvvlee/kirby/server/internal/permission"
)

// UserCreateParam 创建用户入参
type UserCreateParam struct {
	Actor         permission.Actor
	Username      string
	Email         string
	Nickname      string
	PlainPassword string
	SystemAdmin   bool
}

// UserUpdateParam 更新用户入参
type UserUpdateParam struct {
	Actor       permission.Actor
	UserID      int64
	Nickname    string
	SystemAdmin bool
	Version     int64
}

// UpdateUserStatusParam 更新用户状态入参
type UpdateUserStatusParam struct {
	Actor   permission.Actor
	UserID  int64
	Enabled bool
	Version int64
}
