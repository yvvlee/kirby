package converter

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/util/timeutil"
)

// ProjectToProto converts persisted data to an API message without hiding conversion errors.
//
// Parameters: item contains the persisted record.
// Returns: *commonv1.Project with persisted IDs and metadata; an invalid ID or out-of-range version returns an error.
// Throws: none.
func ProjectToProto(item *model.Project) (*commonv1.Project, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	return &commonv1.Project{
		Id:          item.ID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		CreatedBy:   formatActor(item.CreatedBy),
		UpdatedBy:   formatActor(item.UpdatedBy),
		CreatedAt:   timeutil.FormatRFC3339(item.CreatedAt),
		UpdatedAt:   timeutil.FormatRFC3339(item.UpdatedAt),
		Version:     version,
	}, nil
}
