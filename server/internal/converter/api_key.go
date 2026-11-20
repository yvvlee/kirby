package converter

import (
	"strconv"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
)

// APIKeyToProto exposes credential metadata without the stored secret digest.
//
// Parameters: item is the persisted credential.
// Returns: the public credential metadata; optional timestamps retain their absence.
// Throws: none.
func APIKeyToProto(item *model.ProjectAPIKey) *commonv1.ProjectApiKey {
	return &commonv1.ProjectApiKey{
		Id:           item.ID,
		Name:         item.Name,
		CreatedBy:    strconv.FormatInt(item.CreatedBy, 10),
		CreatedAt:    timestamp(item.CreatedAt),
		LastUsedAt:   optionalTime(item.LastUsedAt),
		RevokedAt:    optionalTime(item.RevokedAt),
		SecretSuffix: item.SecretSuffix,
	}
}

func optionalTime(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamp(*value)
}
