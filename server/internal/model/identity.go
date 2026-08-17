package model

import "time"

type Environment struct {
	Meta        `xorm:"extends"`
	Key         string `xorm:"VARCHAR(64) notnull 'key'"`
	Name        string `xorm:"VARCHAR(64) notnull 'name'"`
	Description string `xorm:"VARCHAR(255) notnull default '' 'description'"`
	Enabled     bool   `xorm:"notnull default true 'enabled'"`
}

func (*Environment) TableName() string { return "environments" }

type User struct {
	Meta          `xorm:"extends"`
	Username      string     `xorm:"VARCHAR(128) notnull 'username'"`
	DisplayName   string     `xorm:"VARCHAR(128) notnull 'display_name'"`
	PasswordHash  string     `xorm:"VARCHAR(255) notnull 'password_hash'"`
	Enabled       bool       `xorm:"notnull default true 'enabled'"`
	IsSystemAdmin bool       `xorm:"notnull default false 'is_system_admin'"`
	LastLoginAt   *time.Time `xorm:"'last_login_at'"`
}

func (*User) TableName() string { return "users" }

type Role struct {
	Meta        `xorm:"extends"`
	Key         string `xorm:"VARCHAR(64) notnull 'key'"`
	Name        string `xorm:"VARCHAR(64) notnull 'name'"`
	Description string `xorm:"VARCHAR(255) notnull default '' 'description'"`
	Builtin     bool   `xorm:"notnull default false 'builtin'"`
}

func (*Role) TableName() string { return "roles" }

type Permission struct {
	RecordMeta  `xorm:"extends"`
	Key         string `xorm:"VARCHAR(128) notnull 'key'"`
	Name        string `xorm:"VARCHAR(128) notnull 'name'"`
	Description string `xorm:"VARCHAR(255) notnull default '' 'description'"`
}

func (*Permission) TableName() string { return "permissions" }

type UserEnvironmentRole struct {
	RecordMeta    `xorm:"extends"`
	UserID        int64 `xorm:"notnull 'user_id'"`
	EnvironmentID int64 `xorm:"notnull 'environment_id'"`
	RoleID        int64 `xorm:"notnull 'role_id'"`
	CreatedBy     int64 `xorm:"notnull default 0 'created_by'"`
}

func (*UserEnvironmentRole) TableName() string { return "user_environment_roles" }

type RolePermission struct {
	RecordMeta   `xorm:"extends"`
	RoleID       int64 `xorm:"notnull 'role_id'"`
	PermissionID int64 `xorm:"notnull 'permission_id'"`
	CreatedBy    int64 `xorm:"notnull default 0 'created_by'"`
}

func (*RolePermission) TableName() string { return "role_permissions" }

type RefreshToken struct {
	RecordMeta   `xorm:"extends"`
	UserID       int64      `xorm:"notnull 'user_id'"`
	SessionID    string     `xorm:"VARCHAR(64) notnull 'session_id'"`
	TokenHash    []byte     `xorm:"BINARY(32) notnull 'token_hash'"`
	ExpiresAt    time.Time  `xorm:"notnull 'expires_at'"`
	LastUsedAt   *time.Time `xorm:"'last_used_at'"`
	RevokedAt    *time.Time `xorm:"'revoked_at'"`
	ReplacedByID *int64     `xorm:"'replaced_by_id'"`
}

func (*RefreshToken) TableName() string { return "refresh_tokens" }
