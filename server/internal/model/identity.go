package model

import (
	"time"

	"github.com/yvvlee/lorm"
)

type Environment struct {
	lorm.UnimplementedTable `lorm:"environments"`
	Meta
	ProjectID   int64  `lorm:"project_id"`
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
	Enabled     bool   `lorm:"enabled"`
}

type User struct {
	lorm.UnimplementedTable `lorm:"users"`
	Meta
	Username           string     `lorm:"username"`
	DisplayName        string     `lorm:"display_name"`
	PasswordHash       string     `lorm:"password_hash"`
	Enabled            bool       `lorm:"enabled"`
	IsSystemAdmin      bool       `lorm:"is_system_admin"`
	MustChangePassword bool       `lorm:"must_change_password"`
	LastLoginAt        *time.Time `lorm:"last_login_at"`
}

type Role struct {
	lorm.UnimplementedTable `lorm:"roles"`
	Meta
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
	Builtin     bool   `lorm:"builtin"`
}

type Permission struct {
	lorm.UnimplementedTable `lorm:"permissions"`
	RecordMeta
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
}

type UserEnvironmentRole struct {
	lorm.UnimplementedTable `lorm:"user_environment_roles"`
	RecordMeta
	UserID        int64 `lorm:"user_id"`
	EnvironmentID int64 `lorm:"environment_id"`
	RoleID        int64 `lorm:"role_id"`
	CreatedBy     int64 `lorm:"created_by"`
}

type RolePermission struct {
	lorm.UnimplementedTable `lorm:"role_permissions"`
	RecordMeta
	RoleID       int64 `lorm:"role_id"`
	PermissionID int64 `lorm:"permission_id"`
	CreatedBy    int64 `lorm:"created_by"`
}

type RefreshToken struct {
	lorm.UnimplementedTable `lorm:"refresh_tokens"`
	RecordMeta
	UserID       int64      `lorm:"user_id"`
	SessionID    string     `lorm:"session_id"`
	TokenHash    []byte     `lorm:"token_hash"`
	ExpiresAt    time.Time  `lorm:"expires_at"`
	LastUsedAt   *time.Time `lorm:"last_used_at"`
	RevokedAt    *time.Time `lorm:"revoked_at"`
	ReplacedByID *int64     `lorm:"replaced_by_id"`
}
