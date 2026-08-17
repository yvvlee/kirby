package model

type ImportStatus string

const (
	ImportStatusPending   ImportStatus = "pending"
	ImportStatusSucceeded ImportStatus = "succeeded"
	ImportStatusFailed    ImportStatus = "failed"
)

type ImportRecord struct {
	WorkflowMeta        `xorm:"extends"`
	UserID              int64        `xorm:"notnull 'user_id'"`
	SourceEnvironmentID int64        `xorm:"notnull 'source_environment_id'"`
	TargetEnvironmentID int64        `xorm:"notnull 'target_environment_id'"`
	SourceSnapshotID    int64        `xorm:"notnull 'source_snapshot_id'"`
	TargetProjectID     int64        `xorm:"notnull 'target_project_id'"`
	TargetSnapshotID    *int64       `xorm:"'target_snapshot_id'"`
	IdempotencyKey      string       `xorm:"VARCHAR(128) notnull 'idempotency_key'"`
	RequestHash         []byte       `xorm:"BINARY(32) notnull 'request_hash'"`
	Status              ImportStatus `xorm:"VARCHAR(16) notnull 'status'"`
	ResultJSON          *string      `xorm:"JSON 'result_json'"`
	ErrorMessage        string       `xorm:"VARCHAR(1024) notnull default '' 'error_message'"`
}

func (*ImportRecord) TableName() string { return "import_records" }

type AuditResult string

const (
	AuditResultSucceeded AuditResult = "succeeded"
	AuditResultFailed    AuditResult = "failed"
)

type AuditLog struct {
	RecordMeta    `xorm:"extends"`
	ActorUserID   *int64      `xorm:"'actor_user_id'"`
	EnvironmentID *int64      `xorm:"'environment_id'"`
	Action        string      `xorm:"VARCHAR(128) notnull 'action'"`
	ResourceType  string      `xorm:"VARCHAR(64) notnull 'resource_type'"`
	ResourceID    string      `xorm:"VARCHAR(128) notnull default '' 'resource_id'"`
	Result        AuditResult `xorm:"VARCHAR(16) notnull 'result'"`
	RequestID     string      `xorm:"VARCHAR(128) notnull 'request_id'"`
	DetailsJSON   *string     `xorm:"JSON 'details_json'"`
}

func (*AuditLog) TableName() string { return "audit_logs" }
