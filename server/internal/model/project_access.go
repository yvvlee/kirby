package model

import "github.com/yvvlee/lorm"

// ProjectMember stores one fixed-role membership in a project.
type ProjectMember struct {
	lorm.UnimplementedTable `lorm:"project_members"`
	RecordMeta
	Version   int64  `lorm:"version,version"`
	ProjectID int64  `lorm:"project_id"`
	UserID    int64  `lorm:"user_id"`
	Role      string `lorm:"role"`
	CreatedBy int64  `lorm:"created_by"`
}

// EnvironmentPermissionGrant stores one role permission point for an environment.
type EnvironmentPermissionGrant struct {
	lorm.UnimplementedTable `lorm:"environment_permission_grants"`
	RecordMeta
	Version       int64  `lorm:"version,version"`
	EnvironmentID int64  `lorm:"environment_id"`
	Role          string `lorm:"role"`
	Permission    string `lorm:"permission"`
	CreatedBy     int64  `lorm:"created_by"`
}
