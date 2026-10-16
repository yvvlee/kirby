package converter

import (
	"strconv"
	"time"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// APIKeyToProto exposes credential metadata without the stored secret digest.
//
// Parameters: item is the persisted credential.
// Returns: the public credential metadata; optional timestamps retain their absence.
// Throws: none.
func APIKeyToProto(item *model.ProjectAPIKey) *commonv1.ProjectApiKey {
	return &commonv1.ProjectApiKey{
		Id:           item.ID,
		PublicId:     item.PublicID,
		Name:         item.Name,
		CreatedBy:    strconv.FormatInt(item.CreatedBy, 10),
		CreatedAt:    timeutil.FormatRFC3339(item.CreatedAt),
		LastUsedAt:   optionalTime(item.LastUsedAt),
		RevokedAt:    optionalTime(item.RevokedAt),
		SecretSuffix: item.SecretSuffix,
	}
}

func optionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := timeutil.FormatRFC3339(*value)
	return &formatted
}
