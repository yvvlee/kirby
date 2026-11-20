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

// ApplicationSettings stores singleton system-level switches.
type ApplicationSettings struct {
	lorm.UnimplementedTable `lorm:"application_settings"`
	Meta
	AllowSelfRegistration bool `lorm:"allow_self_registration"`
}

type User struct {
	lorm.UnimplementedTable `lorm:"users"`
	Meta
	Username           string     `lorm:"username"`
	Email              string     `lorm:"email"`
	Nickname           string     `lorm:"nickname"`
	PasswordHash       string     `lorm:"password_hash"`
	Enabled            bool       `lorm:"enabled"`
	IsSystemAdmin      bool       `lorm:"is_system_admin"`
	MustChangePassword bool       `lorm:"must_change_password"`
	LastLoginAt        *time.Time `lorm:"last_login_at"`
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
