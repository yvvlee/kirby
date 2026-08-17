package model

import "time"

type Project struct {
	Meta          `xorm:"extends"`
	EnvironmentID int64  `xorm:"notnull 'environment_id'"`
	Key           string `xorm:"VARCHAR(128) notnull 'key'"`
	Name          string `xorm:"VARCHAR(64) notnull 'name'"`
	Description   string `xorm:"VARCHAR(255) notnull default '' 'description'"`
}

func (*Project) TableName() string { return "projects" }

type ProjectAPIKey struct {
	RecordMeta   `xorm:"extends"`
	ProjectID    int64      `xorm:"notnull 'project_id'"`
	PublicID     string     `xorm:"VARCHAR(64) notnull 'public_id'"`
	Name         string     `xorm:"VARCHAR(64) notnull 'name'"`
	SecretHash   []byte     `xorm:"BINARY(32) notnull 'secret_hash'"`
	SecretSuffix string     `xorm:"CHAR(4) notnull 'secret_suffix'"`
	CreatedBy    int64      `xorm:"notnull 'created_by'"`
	LastUsedAt   *time.Time `xorm:"'last_used_at'"`
	RevokedAt    *time.Time `xorm:"'revoked_at'"`
}

func (*ProjectAPIKey) TableName() string { return "project_api_keys" }

type Config struct {
	Meta           `xorm:"extends"`
	ProjectID      int64  `xorm:"notnull 'project_id'"`
	Key            string `xorm:"VARCHAR(128) notnull 'key'"`
	Description    string `xorm:"VARCHAR(255) notnull default '' 'description'"`
	IsArray        bool   `xorm:"notnull default false 'is_array'"`
	TypeJSON       string `xorm:"JSON notnull 'type_json'"`
	Value          string `xorm:"MEDIUMTEXT notnull 'value'"`
	RuntimeVersion int64  `xorm:"BIGINT notnull default 0 'runtime_version'"`
}

func (*Config) TableName() string { return "configs" }

type Structure struct {
	Meta        `xorm:"extends"`
	ConfigID    int64  `xorm:"notnull 'config_id'"`
	Key         string `xorm:"VARCHAR(128) notnull 'key'"`
	Name        string `xorm:"VARCHAR(64) notnull 'name'"`
	Description string `xorm:"VARCHAR(255) notnull default '' 'description'"`
	FieldsJSON  string `xorm:"JSON notnull 'fields_json'"`
}

func (*Structure) TableName() string { return "structures" }

type ConfigEnum struct {
	Meta        `xorm:"extends"`
	ConfigID    int64  `xorm:"notnull 'config_id'"`
	Key         string `xorm:"VARCHAR(128) notnull 'key'"`
	Name        string `xorm:"VARCHAR(64) notnull 'name'"`
	Description string `xorm:"VARCHAR(255) notnull default '' 'description'"`
	ValuesJSON  string `xorm:"JSON notnull 'values_json'"`
}

func (*ConfigEnum) TableName() string { return "config_enums" }

type SnapshotStatus int8

const (
	SnapshotStatusUnreleased SnapshotStatus = 1
	SnapshotStatusReleased   SnapshotStatus = 3
)

type Snapshot struct {
	Meta        `xorm:"extends"`
	ProjectID   int64          `xorm:"notnull 'project_id'"`
	ConfigID    int64          `xorm:"notnull 'config_id'"`
	ConfigKey   string         `xorm:"VARCHAR(128) notnull 'config_key'"`
	Description string         `xorm:"VARCHAR(255) notnull 'description'"`
	Content     string         `xorm:"MEDIUMTEXT notnull 'content'"`
	Status      SnapshotStatus `xorm:"TINYINT notnull 'status'"`
	TagsJSON    string         `xorm:"JSON notnull 'tags_json'"`
	IsUsing     bool           `xorm:"notnull default false 'is_using'"`
	PublishedAt *time.Time     `xorm:"'published_at'"`
	PublishedBy *int64         `xorm:"'published_by'"`
}

func (*Snapshot) TableName() string { return "snapshots" }
