package converter

import (
	"fmt"

	commonv1 "github.com/yvvlee/kirby/server/api/common"
	"github.com/yvvlee/kirby/server/internal/model"
)

// EnvironmentToProto uses one field mapping for administration and authenticated profile responses.
//
// Parameters: item is the persisted environment, including its project owner.
// Returns: the public environment with project ID, or an error when its ID or version is invalid.
// Throws: none; failures are returned as errors.
func EnvironmentToProto(item *model.Environment) (*commonv1.Environment, error) {
	if item.ID <= 0 {
		return nil, fmt.Errorf("invalid environment record")
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
		Version:     uint32(item.Version),
	}, nil
}
