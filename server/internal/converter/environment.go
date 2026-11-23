package converter

import (
	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
)

// EnvironmentToProto uses one field mapping for administration and authenticated profile responses.
//
// Parameters: item is the persisted environment, including its project owner.
// Returns: the public environment with project ID, or an error when its ID or version is invalid.
// Throws: none; failures are returned as errors.
func EnvironmentToProto(item *model.Environment) (*commonv1.Environment, error) {
	version, err := validMeta(item.ID, item.Version)
	if err != nil {
		return nil, err
	}
	return &commonv1.Environment{
		Id:          item.ID,
		ProjectId:   item.ProjectID,
		Key:         item.Key,
		Name:        item.Name,
		Description: item.Description,
		Enabled:     item.Enabled,
		CreatedAt:   timestamp(item.CreatedAt),
		UpdatedAt:   timestamp(item.UpdatedAt),
		Version:     version,
	}, nil
}
