package model

import "github.com/yvvlee/lorm"

type ImportStatus string

const (
	ImportStatusPending   ImportStatus = "pending"
	ImportStatusSucceeded ImportStatus = "succeeded"
	ImportStatusFailed    ImportStatus = "failed"
)

type ImportRecord struct {
	lorm.UnimplementedTable `lorm:"import_records"`
	WorkflowMeta
	UserID              int64        `lorm:"user_id"`
	SourceEnvironmentID int64        `lorm:"source_environment_id"`
	TargetEnvironmentID int64        `lorm:"target_environment_id"`
	SourceSnapshotID    int64        `lorm:"source_snapshot_id"`
	TargetProjectID     int64        `lorm:"target_project_id"`
	TargetSnapshotID    *int64       `lorm:"target_snapshot_id"`
	IdempotencyKey      string       `lorm:"idempotency_key"`
	RequestHash         []byte       `lorm:"request_hash"`
	Status              ImportStatus `lorm:"status"`
	// JSON columns contain encoded documents; keep their text without encoding a second time.
	ResultJSON   *string `lorm:"result_json"`
	ErrorMessage string  `lorm:"error_message"`
}

type AuditResult string

const (
	AuditResultSucceeded AuditResult = "succeeded"
	AuditResultFailed    AuditResult = "failed"
)

type AuditLog struct {
	lorm.UnimplementedTable `lorm:"audit_logs"`
	RecordMeta
	ActorUserID   *int64      `lorm:"actor_user_id"`
	EnvironmentID *int64      `lorm:"environment_id"`
	Action        string      `lorm:"action"`
	ResourceType  string      `lorm:"resource_type"`
	ResourceID    string      `lorm:"resource_id"`
	Result        AuditResult `lorm:"result"`
	RequestID     string      `lorm:"request_id"`
	// The nullable JSON column keeps SQL NULL distinct from an encoded document.
	DetailsJSON *string `lorm:"details_json"`
}
