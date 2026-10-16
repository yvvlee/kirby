package model

import (
	"time"

	"github.com/yvvlee/lorm"
)

type Project struct {
	lorm.UnimplementedTable `lorm:"projects"`
	Meta
	projectScope
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
}

// lormgen excludes private embedded fields. EnvironmentID is request scope,
// while the persisted project ownership lives in environments.project_id.
type projectScope struct {
	EnvironmentID int64
}

type ProjectAPIKey struct {
	lorm.UnimplementedTable `lorm:"project_api_keys"`
	RecordMeta
	ProjectID    int64      `lorm:"project_id"`
	PublicID     string     `lorm:"public_id"`
	Name         string     `lorm:"name"`
	SecretHash   []byte     `lorm:"secret_hash"`
	SecretSuffix string     `lorm:"secret_suffix"`
	CreatedBy    int64      `lorm:"created_by"`
	LastUsedAt   *time.Time `lorm:"last_used_at"`
	RevokedAt    *time.Time `lorm:"revoked_at"`
}

type Config struct {
	lorm.UnimplementedTable `lorm:"configs"`
	Meta
	ProjectID      int64  `lorm:"project_id"`
	Key            string `lorm:"key"`
	Description    string `lorm:"description"`
	IsArray        bool   `lorm:"is_array"`
	TypeJSON       string `lorm:"type_json"`
	Value          string `lorm:"value"`
	RuntimeVersion int64  `lorm:"runtime_version"`
}

type Structure struct {
	lorm.UnimplementedTable `lorm:"structures"`
	Meta
	ConfigID    int64  `lorm:"config_id"`
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
	FieldsJSON  string `lorm:"fields_json"`
}

type ConfigEnum struct {
	lorm.UnimplementedTable `lorm:"config_enums"`
	Meta
	ConfigID    int64  `lorm:"config_id"`
	Key         string `lorm:"key"`
	Name        string `lorm:"name"`
	Description string `lorm:"description"`
	ValuesJSON  string `lorm:"values_json"`
}

type SnapshotStatus int8

const (
	SnapshotStatusUnreleased SnapshotStatus = 1
	SnapshotStatusReleased   SnapshotStatus = 3
)

type Snapshot struct {
	lorm.UnimplementedTable `lorm:"snapshots"`
	Meta
	ProjectID   int64          `lorm:"project_id"`
	ConfigID    int64          `lorm:"config_id"`
	ConfigKey   string         `lorm:"config_key"`
	Description string         `lorm:"description"`
	Content     string         `lorm:"content"`
	Status      SnapshotStatus `lorm:"status"`
	TagsJSON    string         `lorm:"tags_json"`
	IsUsing     bool           `lorm:"is_using"`
	PublishedAt *time.Time     `lorm:"published_at"`
	PublishedBy *int64         `lorm:"published_by"`
}
