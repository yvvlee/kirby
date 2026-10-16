// Package model defines the database records used by Kirby.
//
// The generated lorm metadata maps records to the existing SQL schema.
package model

import (
	"time"

	"github.com/yvvlee/lorm"
)

//go:generate go tool lormgen .

// Meta is shared by mutable business resources. DeletedAt enables explicit
// soft deletion without losing audit or ownership information.
type Meta struct {
	ID        int64      `lorm:"id,primary_key,auto_increment"`
	CreatedBy int64      `lorm:"created_by"`
	UpdatedBy int64      `lorm:"updated_by"`
	CreatedAt time.Time  `lorm:"created_at,created"`
	UpdatedAt time.Time  `lorm:"updated_at,updated"`
	Version   int64      `lorm:"version,version"`
	DeletedAt *time.Time `lorm:"deleted_at"`
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

// PrepareInsert starts optimistic locking at one and stores timezone-free SQL
// timestamps in UTC so PostgreSQL and MySQL represent the same instant.
//
// Parameters: none; existing creation and modification instants are retained.
// Returns: none; missing timestamps and the initial version are filled in place.
// Throws: none.
func (m *Meta) PrepareInsert() {
	m.CreatedAt, m.UpdatedAt = insertTimes(m.CreatedAt, m.UpdatedAt)
	m.Version = 1
}

// PrepareInsert normalizes immutable-record timestamps before lorm's local-time
// hooks can fill them, preserving the UTC convention of the SQL schema.
//
// Parameters: none; existing creation and modification instants are retained.
// Returns: none; missing timestamps are filled in place.
// Throws: none.
func (m *RecordMeta) PrepareInsert() {
	m.CreatedAt, m.UpdatedAt = insertTimes(m.CreatedAt, m.UpdatedAt)
}

// PrepareInsert initializes workflow versions and normalizes SQL timestamps to
// UTC without changing an explicitly supplied creation or modification instant.
//
// Parameters: none.
// Returns: none; missing timestamps and the initial version are filled in place.
// Throws: none.
func (m *WorkflowMeta) PrepareInsert() {
	m.CreatedAt, m.UpdatedAt = insertTimes(m.CreatedAt, m.UpdatedAt)
	m.Version = 1
}

func insertTimes(createdAt, updatedAt time.Time) (time.Time, time.Time) {
	now := time.Now().UTC()
	if createdAt.IsZero() {
		createdAt = now
	}
	if updatedAt.IsZero() {
		updatedAt = now
	}
	return createdAt.UTC(), updatedAt.UTC()
}

// AllModels is the inventory of tables mapped by generated lorm metadata.
var AllModels = []lorm.Table{
	&ApplicationSecrets{},
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
