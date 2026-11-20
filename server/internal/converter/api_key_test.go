package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yvvlee/kirby/server/internal/model"
)

func TestProjectAPIKeyProtoNeverContainsDigest(t *testing.T) {
	now := time.Date(2026, time.August, 17, 8, 0, 0, 0, time.UTC)
	item := model.ProjectAPIKey{
		RecordMeta: model.RecordMeta{
			ID:        41,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ProjectID:    20,
		Name:         "production",
		SecretHash:   []byte("database-only-digest"),
		SecretSuffix: "abcd",
		CreatedBy:    9,
	}
	converted := APIKeyToProto(&item)
	assert.Equal(t, int64(41), converted.Id)
	assert.Equal(t, "abcd", converted.SecretSuffix)
	assert.NotContains(t, converted.String(), "database-only-digest")
}
