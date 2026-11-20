package model

import (
	"time"

	"github.com/yvvlee/lorm"
)

// ProjectInvitation stores a single-use invitation token hash.
type ProjectInvitation struct {
	lorm.UnimplementedTable `lorm:"project_invitations"`
	RecordMeta
	ProjectID  int64      `lorm:"project_id"`
	Role       string     `lorm:"role"`
	TokenHash  []byte     `lorm:"token_hash"`
	CreatedBy  int64      `lorm:"created_by"`
	ExpiresAt  time.Time  `lorm:"expires_at"`
	AcceptedBy *int64     `lorm:"accepted_by"`
	AcceptedAt *time.Time `lorm:"accepted_at"`
	RevokedAt  *time.Time `lorm:"revoked_at"`
}
