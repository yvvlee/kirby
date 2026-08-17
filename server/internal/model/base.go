// Package model defines the database records used by Kirby.
//
// deploy/schema.sql is the only schema definition. These types describe rows
// for xorm; they must never be passed to schema synchronization helpers.
package model

import "time"

// Meta is shared by mutable business resources. DeletedAt enables explicit
// soft deletion without losing audit or ownership information.
type Meta struct {
	ID        int64     `xorm:"pk autoincr 'id'"`
	CreatedBy int64     `xorm:"notnull default 0 'created_by'"`
	UpdatedBy int64     `xorm:"notnull default 0 'updated_by'"`
	CreatedAt time.Time `xorm:"created notnull 'created_at'"`
	UpdatedAt time.Time `xorm:"updated notnull 'updated_at'"`
	Version   int64     `xorm:"version notnull default 0 'version'"`
	DeletedAt time.Time `xorm:"deleted 'deleted_at'"`
}

// RecordMeta is shared by immutable records and relationship records.
type RecordMeta struct {
	ID        int64     `xorm:"pk autoincr 'id'"`
	CreatedAt time.Time `xorm:"created notnull 'created_at'"`
	UpdatedAt time.Time `xorm:"updated notnull 'updated_at'"`
}

// WorkflowMeta is used by mutable records that must not be soft-deleted.
type WorkflowMeta struct {
	ID        int64     `xorm:"pk autoincr 'id'"`
	CreatedBy int64     `xorm:"notnull default 0 'created_by'"`
	UpdatedBy int64     `xorm:"notnull default 0 'updated_by'"`
	CreatedAt time.Time `xorm:"created notnull 'created_at'"`
	UpdatedAt time.Time `xorm:"updated notnull 'updated_at'"`
	Version   int64     `xorm:"version notnull default 0 'version'"`
}

// AllModels is an inventory for repository wiring and schema checks. It is
// intentionally not used for automatic table creation.
var AllModels = []any{
	&Environment{},
	&User{},
	&Role{},
	&Permission{},
	&UserEnvironmentRole{},
	&RolePermission{},
	&RefreshToken{},
	&Project{},
	&ProjectAPIKey{},
	&Config{},
	&Structure{},
	&ConfigEnum{},
	&Snapshot{},
	&ImportRecord{},
	&AuditLog{},
}
