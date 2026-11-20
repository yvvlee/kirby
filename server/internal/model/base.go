// Package model defines the database records used by Kirby.
//
// The generated lorm metadata maps records to the existing SQL schema.
package model

import (
	"time"

	"github.com/yvvlee/lorm"
)

//go:generate go tool lormgen .

// Meta is shared by mutable business resources.
type Meta struct {
	ID        int64     `lorm:"id,primary_key,auto_increment"`
	CreatedBy int64     `lorm:"created_by"`
	UpdatedBy int64     `lorm:"updated_by"`
	CreatedAt time.Time `lorm:"created_at,created"`
	UpdatedAt time.Time `lorm:"updated_at,updated"`
	Version   int64     `lorm:"version,version"`
}

// RecordMeta is shared by immutable records and relationship records.
type RecordMeta struct {
	ID        int64     `lorm:"id,primary_key,auto_increment"`
	CreatedAt time.Time `lorm:"created_at,created"`
	UpdatedAt time.Time `lorm:"updated_at,updated"`
}

// WorkflowMeta is used by mutable records that must not be soft-deleted.
type WorkflowMeta struct {
	ID        int64     `lorm:"id,primary_key,auto_increment"`
	CreatedBy int64     `lorm:"created_by"`
	UpdatedBy int64     `lorm:"updated_by"`
	CreatedAt time.Time `lorm:"created_at,created"`
	UpdatedAt time.Time `lorm:"updated_at,updated"`
	Version   int64     `lorm:"version,version"`
}

// AllModels is the inventory of tables mapped by generated lorm metadata.
var AllModels = []lorm.Table{
	&ApplicationSecrets{},
	&ApplicationSettings{},
	&Environment{},
	&User{},
	&RefreshToken{},
	&Project{},
	&ProjectAPIKey{},
	&Config{},
	&Structure{},
	&ConfigEnum{},
	&Snapshot{},
	&ImportRecord{},
	&AuditLog{},
	&ProjectMember{},
	&EnvironmentPermissionGrant{},
	&ProjectInvitation{},
}
